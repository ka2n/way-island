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
)

const terminalAppID = "Alacritty"

var (
	errSessionNotFound = errors.New("session not found")
	errNoClaudePID     = errors.New("session has no Claude PID")
	errPaneNotFound    = errors.New("tmux pane not found")
	errNoTmuxClient    = errors.New("tmux client not found")
)

type focusRunner func(name string, args ...string) ([]byte, error)
type parentPIDReader func(pid int) (int, error)
type sleepFunc func(time.Duration)

type tmuxPane struct {
	PanePID     int
	SessionName string
	WindowName  string
	PaneID      string
}

type sessionFocuser struct {
	store      *overlayModel
	runCommand focusRunner
	parentPID  parentPIDReader
	sleep      sleepFunc
}

func newSessionFocuser(store *overlayModel) *sessionFocuser {
	return &sessionFocuser{
		store:      store,
		runCommand: runFocusCommand,
		parentPID:  readParentPID,
		sleep:      time.Sleep,
	}
}

func (f *sessionFocuser) Focus(sessionID string) error {
	session, ok := f.store.Session(sessionID)
	if !ok {
		return fmt.Errorf("%w: %s", errSessionNotFound, sessionID)
	}
	if session.ClaudePID <= 0 {
		return fmt.Errorf("%w: %s", errNoClaudePID, sessionID)
	}

	pane, err := f.resolvePane(session.ClaudePID)
	if err != nil {
		return err
	}

	if err := f.focusTmux(pane); err != nil {
		return err
	}

	if pane.WindowName == "" {
		return nil
	}

	f.sleep(50 * time.Millisecond)
	if _, err := f.runCommand("wlrctl", "toplevel", "focus", "app_id:"+terminalAppID, "title:"+pane.WindowName); err != nil {
		return fmt.Errorf("focus terminal window %q: %w", pane.WindowName, err)
	}

	return nil
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
	output, err := f.runCommand("tmux", "list-panes", "-a", "-F", "#{pane_pid}\t#{session_name}\t#{window_name}\t#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("list tmux panes: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	panes := make([]tmuxPane, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
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
		})
	}

	return panes, nil
}

func (f *sessionFocuser) focusTmux(pane tmuxPane) error {
	clients, err := f.listTmuxClients(pane.SessionName)
	if err != nil {
		return err
	}
	if len(clients) == 0 {
		return fmt.Errorf("%w for session %q", errNoTmuxClient, pane.SessionName)
	}
	if pane.PaneID != "" {
		if _, err := f.runCommand("tmux", "switch-client", "-c", clients[0], "-t", pane.PaneID); err != nil {
			return fmt.Errorf("switch tmux client %q to pane %q: %w", clients[0], pane.PaneID, err)
		}
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
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "PPid:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "PPid:"))
		parent, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return parent, nil
	}

	return 0, fmt.Errorf("PPid not found for pid %d", pid)
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

func triggerSessionFocus(focuser *sessionFocuser, sessionID string) {
	go func() {
		if err := focuser.Focus(sessionID); err != nil {
			log.Printf("focus session %s: %v", sessionID, err)
		}
	}()
}
