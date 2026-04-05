package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ka2n/way-island/internal/socket"
)

type parentPIDReader func(pid int) (int, error)

type tmuxPane struct {
	PanePID     int
	SessionName string
	WindowName  string
	PaneID      string
	PaneTTY     string
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

func (f *sessionFocuser) focusTmux(pane tmuxPane) (string, error) {
	clients, err := f.listTmuxClients(pane.SessionName)
	if err != nil {
		return "", err
	}
	debugf("focus tmux clients session=%s clients=%v", pane.SessionName, clients)
	if len(clients) == 0 {
		return "", fmt.Errorf("%w for session %q", errNoTmuxClient, pane.SessionName)
	}
	clientTTY := clients[0]
	if pane.PaneID != "" {
		if _, err := f.runCommand("tmux", "switch-client", "-c", clientTTY, "-t", pane.PaneID); err != nil {
			return "", fmt.Errorf("switch tmux client %q to pane %q: %w", clientTTY, pane.PaneID, err)
		}
		debugf("focus tmux switched client client=%s pane=%s", clientTTY, pane.PaneID)
		if err := f.refreshTmuxClient(clientTTY, pane.PaneID); err != nil {
			return "", err
		}
	}

	return clientTTY, nil
}

func (f *sessionFocuser) refreshTmuxClient(clientTTY string, paneID string) error {
	if strings.TrimSpace(clientTTY) == "" {
		return nil
	}
	if _, err := f.runCommand("tmux", "refresh-client", "-t", clientTTY); err != nil {
		return fmt.Errorf("refresh tmux client %q after switching pane %q: %w", clientTTY, paneID, err)
	}
	debugf("focus tmux refreshed client client=%s pane=%s", clientTTY, paneID)
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
