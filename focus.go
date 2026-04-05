package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

const terminalAppID = "Alacritty"

var (
	errSessionNotFound = errors.New("session not found")
	errNoAgentPID      = errors.New("session has no agent PID")
	errPaneNotFound    = errors.New("tmux pane not found")
	errNoTmuxClient    = errors.New("tmux client not found")
	errHostPIDNotFound = errors.New("host pid not found")
)

type focusRunner func(name string, args ...string) ([]byte, error)
type sleepFunc func(time.Duration)

type sessionFocuser struct {
	store                 *overlayModel
	runCommand            focusRunner
	focusWindow           func(appID string, title string) error
	writeTTY              func(path string, data []byte) error
	parentPID             parentPIDReader
	sleep                 sleepFunc
	readCurrentPIDNSInode func() (uint64, error)
	readPIDNSInode        func(int) (uint64, error)
	readStartTimeTicks    func(int) (uint64, error)
	listProcPIDs          func() ([]int, error)
	readNSPIDs            func(int) ([]int, error)
	focusConfig           focusConfig
}

func newSessionFocuser(store *overlayModel) *sessionFocuser {
	cfg, err := loadAppConfig()
	if err != nil {
		log.Printf("load app config: %v", err)
	}

	return &sessionFocuser{
		store:                 store,
		runCommand:            runFocusCommand,
		focusWindow:           focusTerminalWindow,
		writeTTY:              writeTTYBytes,
		parentPID:             readParentPID,
		sleep:                 time.Sleep,
		readCurrentPIDNSInode: readCurrentPIDNamespaceInode,
		readPIDNSInode:        readPIDNamespaceInodeForPID,
		readStartTimeTicks: func(pid int) (uint64, error) {
			stat, err := readProcStat(pid)
			if err != nil {
				return 0, err
			}
			return stat.StartTimeTicks, nil
		},
		listProcPIDs: listProcPIDs,
		readNSPIDs:   readNSPIDsForPID,
		focusConfig:  cfg.Focus,
	}
}

func (f *sessionFocuser) Focus(sessionID string) error {
	log.Printf("focus session start session_id=%s", sessionID)
	session, ok := f.store.Session(sessionID)
	if !ok {
		return fmt.Errorf("%w: %s", errSessionNotFound, sessionID)
	}
	if session.AgentPID <= 0 {
		return fmt.Errorf("%w: %s", errNoAgentPID, sessionID)
	}

	hostPID, err := f.resolveHostAgentPID(session)
	if err != nil && !errors.Is(err, errHostPIDNotFound) {
		return err
	}
	debugf("focus session resolve host pid session_id=%s agent_pid=%d host_pid=%d err=%v", sessionID, session.AgentPID, hostPID, err)

	pane, err := f.resolvePaneForSession(session, hostPID)
	if err != nil {
		return err
	}
	debugf("focus session resolved pane session_id=%s pane_id=%s pane_tty=%s session=%s window=%s pane_pid=%d",
		sessionID, pane.PaneID, pane.PaneTTY, pane.SessionName, pane.WindowName, pane.PanePID)

	clientTTY, err := f.focusTmux(pane)
	if err != nil {
		return err
	}

	if pane.WindowName == "" {
		if err := f.refreshTmuxClient(clientTTY, pane.PaneID); err != nil {
			return err
		}
		log.Printf("focus session ok session_id=%s pane_id=%s session=%s", sessionID, pane.PaneID, pane.SessionName)
		return nil
	}

	focusWindow := f.focusWindow
	if focusWindow == nil {
		focusWindow = focusTerminalWindow
	}

	// Prefer a stable "session:window" title first so terminals configured with
	// tmux-driven titles can be matched before falling back to looser heuristics.
	targetTitle := preferredTerminalTitle(pane)
	debugf("focus session terminal target session_id=%s app_id=%s title=%q", sessionID, terminalAppID, targetTitle)
	f.sleep(50 * time.Millisecond)
	if err := focusWindow(terminalAppID, targetTitle); err != nil {
		if retryErr := f.retryFocusAfterRetitle(pane, targetTitle, focusWindow, err); retryErr == nil {
			if err := f.refreshTmuxClient(clientTTY, pane.PaneID); err != nil {
				return err
			}
			log.Printf("focus session ok session_id=%s pane_id=%s title=%q retry=true", sessionID, pane.PaneID, targetTitle)
			return nil
		}

		return fmt.Errorf("focus terminal window %q: %w", pane.WindowName, err)
	}

	if err := f.refreshTmuxClient(clientTTY, pane.PaneID); err != nil {
		return err
	}
	log.Printf("focus session ok session_id=%s pane_id=%s title=%q", sessionID, pane.PaneID, targetTitle)
	return nil
}

func (f *sessionFocuser) capturePaneText(sessionID string) (string, error) {
	session, ok := f.store.Session(sessionID)
	if !ok {
		return "", fmt.Errorf("%w: %s", errSessionNotFound, sessionID)
	}
	if session.AgentPID <= 0 {
		return "", fmt.Errorf("%w: %s", errNoAgentPID, sessionID)
	}

	hostPID, err := f.resolveHostAgentPID(session)
	if err != nil && !errors.Is(err, errHostPIDNotFound) {
		return "", err
	}

	pane, err := f.resolvePaneForSession(session, hostPID)
	if err != nil {
		return "", err
	}
	if pane.PaneID == "" {
		return "", fmt.Errorf("%w for session %q", errPaneNotFound, sessionID)
	}

	output, err := f.runCommand("tmux", "capture-pane", "-p", "-t", pane.PaneID)
	if err != nil {
		return "", fmt.Errorf("capture tmux pane %q: %w", pane.PaneID, err)
	}
	return string(output), nil
}

func (f *sessionFocuser) resolveHostAgentPID(session socket.Session) (int, error) {
	resolver := socket.HostPIDResolver{
		ReadCurrentPIDNSInode: f.readCurrentPIDNSInode,
		ReadPIDNamespaceInode: f.readPIDNSInode,
		ReadNamespacedPIDs:    f.readNSPIDs,
		ReadStartTimeTicks:    f.readStartTimeTicks,
		ListPIDs:              f.listProcPIDs,
	}
	if hostPID, ok := resolver.Resolve(session); ok {
		return hostPID, nil
	}

	return 0, fmt.Errorf("%w: session=%s pid=%d", errHostPIDNotFound, session.ID, session.AgentPID)
}

func (f *sessionFocuser) retryFocusAfterRetitle(pane tmuxPane, title string, focusWindow func(appID string, title string) error, focusErr error) error {
	if !f.focusConfig.RetitleWithOSC {
		return focusErr
	}
	if strings.TrimSpace(pane.PaneTTY) == "" || strings.TrimSpace(title) == "" {
		return focusErr
	}
	if !strings.Contains(focusErr.Error(), "wayland toplevel not found") {
		return focusErr
	}
	// Only rewrite the terminal title when Wayland matching failed; this keeps
	// the OSC path as an opt-in recovery mechanism instead of the primary path.
	log.Printf("focus session osc retitle pane_tty=%s title=%q", pane.PaneTTY, title)
	if err := f.writeOSCWindowTitle(pane.PaneTTY, title); err != nil {
		return err
	}
	debugf("focus session osc retitle written pane_tty=%s title=%q", pane.PaneTTY, title)

	f.sleep(50 * time.Millisecond)
	return focusWindow(terminalAppID, title)
}

func (f *sessionFocuser) writeOSCWindowTitle(ttyPath, title string) error {
	if f.writeTTY == nil {
		return errors.New("tty writer unavailable")
	}

	safeTitle := sanitizeOSCTitle(title)
	payload := []byte(fmt.Sprintf("\033]0;%s\007\033]2;%s\007", safeTitle, safeTitle))
	if err := f.writeTTY(ttyPath, payload); err != nil {
		return fmt.Errorf("write osc title to %q: %w", ttyPath, err)
	}

	return nil
}

func preferredTerminalTitle(pane tmuxPane) string {
	session := strings.TrimSpace(pane.SessionName)
	window := strings.TrimSpace(pane.WindowName)
	switch {
	case session != "" && window != "":
		return session + ":" + window
	case window != "":
		return window
	default:
		return session
	}
}

func sanitizeOSCTitle(title string) string {
	// Strip control bytes so a tmux window name cannot break the OSC sequence.
	return strings.Map(func(r rune) rune {
		switch r {
		case '\a', '\x1b':
			return -1
		default:
			if r < 0x20 {
				return -1
			}
			return r
		}
	}, title)
}

func triggerSessionFocus(focuser *sessionFocuser, sessionID string) {
	go func() {
		if err := focuser.Focus(sessionID); err != nil {
			log.Printf("focus session %s: %v", sessionID, err)
		}
	}()
}

func stringField(fields []string, index int) string {
	if index >= len(fields) {
		return ""
	}
	return fields[index]
}

func ttyMatches(a string, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	return a == b || ttyBaseName(a) == ttyBaseName(b)
}
