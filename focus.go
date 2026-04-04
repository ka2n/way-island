package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
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
type parentPIDReader func(pid int) (int, error)
type sleepFunc func(time.Duration)

type tmuxPane struct {
	PanePID     int
	SessionName string
	WindowName  string
	PaneID      string
	PaneTTY     string
}

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

	if err := f.focusTmux(pane); err != nil {
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
	debugf("focus session terminal target session_id=%s app_id=%s title=%q", sessionID, terminalAppID, targetTitle)
	f.sleep(50 * time.Millisecond)
	if err := focusWindow(terminalAppID, targetTitle); err != nil {
		if retryErr := f.retryFocusAfterRetitle(pane, targetTitle, focusWindow, err); retryErr == nil {
			log.Printf("focus session ok session_id=%s pane_id=%s title=%q retry=true", sessionID, pane.PaneID, targetTitle)
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

func (f *sessionFocuser) resolvePaneForSession(session socket.Session, hostPID int) (tmuxPane, error) {
	// PID ancestry is the strongest signal; tty matching keeps focus working when
	// the agent PID cannot be resolved across namespaces.
	if pane, err := f.resolvePane(hostPID); err == nil {
		return pane, nil
	}

	return f.resolvePaneByTTY(session)
}

func (f *sessionFocuser) resolvePane(pid int) (tmuxPane, error) {
	ancestors, err := collectAncestorPIDs(pid, f.parentPID)
	if err != nil {
		return tmuxPane{}, fmt.Errorf("collect ancestor pids for %d: %w", pid, err)
	}

	panes, err := f.listTmuxPanes()
	if err != nil {
		return tmuxPane{}, err
	}

	bestDepth := len(ancestors) + 1
	var best tmuxPane
	found := false

	for _, pane := range panes {
		depth, ok := ancestors[pane.PanePID]
		if !ok {
			continue
		}
		if !found || depth < bestDepth {
			best = pane
			bestDepth = depth
			found = true
		}
	}

	if !found {
		return tmuxPane{}, fmt.Errorf("%w for pid %d", errPaneNotFound, pid)
	}

	return best, nil
}

func (f *sessionFocuser) listTmuxPanes() ([]tmuxPane, error) {
	output, err := f.runCommand("tmux", "list-panes", "-a", "-F", "#{pane_pid}\t#{session_name}\t#{window_name}\t#{pane_id}\t#{pane_tty}")
	if err != nil {
		return nil, fmt.Errorf("list tmux panes: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	panes := make([]tmuxPane, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 4 {
			continue
		}
		panePID, err := strconv.Atoi(fields[0])
		if err != nil || panePID <= 0 {
			continue
		}
		panes = append(panes, tmuxPane{
			PanePID:     panePID,
			SessionName: fields[1],
			WindowName:  fields[2],
			PaneID:      fields[3],
			PaneTTY:     stringField(fields, 4),
		})
	}

	return panes, nil
}

func (f *sessionFocuser) resolvePaneByTTY(session socket.Session) (tmuxPane, error) {
	panes, err := f.listTmuxPanes()
	if err != nil {
		return tmuxPane{}, err
	}

	candidates := []string{session.AgentTTY, session.HookTTY}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		for _, pane := range panes {
			if ttyMatches(candidate, pane.PaneTTY) {
				return pane, nil
			}
		}
	}

	return tmuxPane{}, fmt.Errorf("%w via tty", errPaneNotFound)
}

func (f *sessionFocuser) focusTmux(pane tmuxPane) error {
	clients, err := f.listTmuxClients(pane.SessionName)
	if err != nil {
		return err
	}
	debugf("focus tmux clients session=%s clients=%v", pane.SessionName, clients)
	if len(clients) == 0 {
		return fmt.Errorf("%w for session %q", errNoTmuxClient, pane.SessionName)
	}
	if pane.PaneID != "" {
		if _, err := f.runCommand("tmux", "switch-client", "-c", clients[0], "-t", pane.PaneID); err != nil {
			return fmt.Errorf("switch tmux client %q to pane %q: %w", clients[0], pane.PaneID, err)
		}
		debugf("focus tmux switched client client=%s pane=%s", clients[0], pane.PaneID)
		if _, err := f.runCommand("tmux", "refresh-client", "-t", clients[0]); err != nil {
			return fmt.Errorf("refresh tmux client %q after switching pane %q: %w", clients[0], pane.PaneID, err)
		}
		debugf("focus tmux refreshed client client=%s", clients[0])
	}

	return nil
}

func (f *sessionFocuser) listTmuxClients(sessionName string) ([]string, error) {
	if sessionName == "" {
		return nil, fmt.Errorf("%w for empty session name", errNoTmuxClient)
	}

	output, err := f.runCommand("tmux", "list-clients", "-t", sessionName, "-F", "#{client_tty}")
	if err != nil {
		return nil, fmt.Errorf("list tmux clients for %q: %w", sessionName, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	clients := make([]string, 0, len(lines))
	for _, line := range lines {
		client := strings.TrimSpace(line)
		if client == "" {
			continue
		}
		clients = append(clients, client)
	}

	return clients, nil
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

func collectAncestorPIDs(pid int, readParent parentPIDReader) (map[int]int, error) {
	ancestors := map[int]int{}
	current := pid
	for depth := 0; current > 1 && depth < 128; depth++ {
		if _, seen := ancestors[current]; seen {
			break
		}
		ancestors[current] = depth

		parent, err := readParent(current)
		if err != nil {
			return nil, err
		}
		if parent <= 0 || parent == current {
			break
		}
		current = parent
	}
	return ancestors, nil
}

func readParentPID(pid int) (int, error) {
	stat, err := readProcStat(pid)
	if err != nil {
		return 0, err
	}
	return stat.PPid, nil
}

func runFocusCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, nil
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return output, err
	}

	return output, fmt.Errorf("%w: %s", err, trimmed)
}

func writeTTYBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0)
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
