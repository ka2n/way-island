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
	errHostPIDNotFound = errors.New("host pid not found")
)

type focusRunner func(name string, args ...string) ([]byte, error)
type sleepFunc func(time.Duration)

type sessionFocuser struct {
	store                 *overlayModel
	runCommand            focusRunner
	focusWindow           func(appID string, title string) error
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

	if _, err := f.focusTmux(pane); err != nil {
		return err
	}

	if pane.WindowName == "" {
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
	log.Printf("focus wayland attempt session_id=%s app_id=%s title=%q", sessionID, terminalAppID, targetTitle)
	f.sleep(50 * time.Millisecond)
	if err := focusWindow(terminalAppID, targetTitle); err != nil {
		log.Printf("focus wayland failed session_id=%s title=%q err=%v", sessionID, targetTitle, err)
		if !strings.Contains(err.Error(), "wayland toplevel not found") {
			return fmt.Errorf("focus terminal window %q: %w", pane.WindowName, err)
		}
		if retryErr := f.retryFocusWithTmuxTitles(pane.SessionName, targetTitle, focusWindow); retryErr == nil {
			log.Printf("focus session ok session_id=%s pane_id=%s title=%q retry=set-titles", sessionID, pane.PaneID, targetTitle)
			return nil
		}

		return fmt.Errorf("focus terminal window %q: %w", pane.WindowName, err)
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

// retryFocusWithTmuxTitles temporarily enables tmux set-titles so that tmux
// sends an OSC title sequence to the terminal, then retries the Wayland focus.
// The original set-titles and set-titles-string values are always restored.
// retryFocusWithTmuxTitles temporarily enables tmux set-titles and tries
// switching each client to the target session until a Wayland toplevel with
// the expected title appears.  Original set-titles values and each client's
// original session are always restored.
func (f *sessionFocuser) retryFocusWithTmuxTitles(sessionName string, title string, focusWindow func(appID string, title string) error) error {
	if !f.focusConfig.TmuxSetTitles {
		log.Printf("focus set-titles retry skip reason=disabled")
		return errors.New("tmux_set_titles not enabled")
	}

	origTitles, err := f.getTmuxGlobalOption("set-titles")
	if err != nil {
		log.Printf("focus set-titles retry skip reason=get_set-titles err=%v", err)
		return err
	}
	origTitleString, err := f.getTmuxGlobalOption("set-titles-string")
	if err != nil {
		log.Printf("focus set-titles retry skip reason=get_set-titles-string err=%v", err)
		return err
	}
	log.Printf("focus set-titles retry title=%q orig_set-titles=%q orig_set-titles-string=%q", title, origTitles, origTitleString)

	// Cycle set-titles off→on with the desired format to force tmux to
	// re-push the title to all terminals, even when the settings haven't
	// changed from their current values.
	titleStringFormat := "#S:#W"
	f.runCommand("tmux", "set-option", "-g", "set-titles", "off")
	if _, err := f.runCommand("tmux", "set-option", "-g", "set-titles-string", titleStringFormat); err != nil {
		f.restoreTmuxTitles(origTitles, origTitleString)
		return fmt.Errorf("set tmux set-titles-string: %w", err)
	}
	if _, err := f.runCommand("tmux", "set-option", "-g", "set-titles", "on"); err != nil {
		f.restoreTmuxTitles(origTitles, origTitleString)
		return fmt.Errorf("set tmux set-titles on: %w", err)
	}

	allClients, err := f.listTmuxClients("")
	if err != nil {
		f.restoreTmuxTitles(origTitles, origTitleString)
		return err
	}

	// Record each client's original session so we can restore them.
	type clientState struct {
		tty     string
		session string
	}
	var switched []clientState
	defer func() {
		for _, cs := range switched {
			if _, err := f.runCommand("tmux", "switch-client", "-c", cs.tty, "-t", cs.session); err != nil {
				log.Printf("focus set-titles restore client=%s session=%s err=%v", cs.tty, cs.session, err)
			}
		}
		f.restoreTmuxTitles(origTitles, origTitleString)
	}()

	for _, clientTTY := range allClients {
		clientSession, err := f.getTmuxClientSession(clientTTY)
		if err != nil {
			continue
		}
		if clientSession == sessionName {
			// Already on target session — title should already match after select-window.
			f.sleep(50 * time.Millisecond)
			if err := focusWindow(terminalAppID, title); err == nil {
				return nil
			}
			continue
		}
		// Switch this client to the target session.
		log.Printf("focus set-titles retry switch-client client=%s from=%s to=%s", clientTTY, clientSession, sessionName)
		if _, err := f.runCommand("tmux", "switch-client", "-c", clientTTY, "-t", sessionName); err != nil {
			continue
		}
		switched = append(switched, clientState{tty: clientTTY, session: clientSession})

		f.sleep(50 * time.Millisecond)
		if err := focusWindow(terminalAppID, title); err == nil {
			// Found it — remove this client from the restore list since it should stay.
			switched = switched[:len(switched)-1]
			return nil
		}
	}

	log.Printf("focus set-titles retry failed title=%q", title)
	return fmt.Errorf("wayland toplevel not found after trying all tmux clients for title %q", title)
}

func (f *sessionFocuser) getTmuxClientSession(clientTTY string) (string, error) {
	output, err := f.runCommand("tmux", "display-message", "-p", "-t", clientTTY, "-F", "#{session_name}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (f *sessionFocuser) restoreTmuxTitles(origTitles, origTitleString string) {
	if _, err := f.runCommand("tmux", "set-option", "-g", "set-titles", origTitles); err != nil {
		log.Printf("focus set-titles restore failed option=set-titles value=%q err=%v", origTitles, err)
	}
	if _, err := f.runCommand("tmux", "set-option", "-g", "set-titles-string", origTitleString); err != nil {
		log.Printf("focus set-titles restore failed option=set-titles-string value=%q err=%v", origTitleString, err)
	}
}

func (f *sessionFocuser) getTmuxGlobalOption(name string) (string, error) {
	output, err := f.runCommand("tmux", "show-options", "-gv", name)
	if err != nil {
		return "", fmt.Errorf("get tmux option %q: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
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
