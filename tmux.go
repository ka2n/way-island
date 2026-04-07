package main

import (
	"fmt"
	"log"
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
	if pane.PaneID == "" {
		return "", nil
	}
	// If no client is attached to the target session, switch an existing client
	// from another session so the pane becomes visible.
	clientTTY, err := f.ensureTmuxClientAttached(pane.SessionName)
	if err != nil {
		return "", err
	}
	if _, err := f.runCommand("tmux", "select-window", "-t", pane.PaneID); err != nil {
		return "", fmt.Errorf("select tmux window for pane %q: %w", pane.PaneID, err)
	}
	if _, err := f.runCommand("tmux", "select-pane", "-t", pane.PaneID); err != nil {
		return "", fmt.Errorf("select tmux pane %q: %w", pane.PaneID, err)
	}
	debugf("focus tmux selected pane=%s client=%s", pane.PaneID, clientTTY)
	return clientTTY, nil
}

func (f *sessionFocuser) ensureTmuxClientAttached(sessionName string) (string, error) {
	clients, err := f.listTmuxClients(sessionName)
	if err != nil {
		return "", err
	}
	if len(clients) > 0 {
		return clients[0], nil
	}
	// No client on this session — find any client and switch it over.
	allClients, err := f.listTmuxClients("")
	if err != nil {
		return "", err
	}
	if len(allClients) == 0 {
		return "", fmt.Errorf("no tmux clients available to switch to session %q", sessionName)
	}
	clientTTY := allClients[0]
	log.Printf("focus tmux switch-client client=%s session=%s", clientTTY, sessionName)
	if _, err := f.runCommand("tmux", "switch-client", "-c", clientTTY, "-t", sessionName); err != nil {
		return "", fmt.Errorf("switch tmux client %q to session %q: %w", clientTTY, sessionName, err)
	}
	return clientTTY, nil
}

func (f *sessionFocuser) listTmuxClients(sessionName string) ([]string, error) {
	args := []string{"list-clients", "-F", "#{client_tty}"}
	if sessionName != "" {
		args = []string{"list-clients", "-t", sessionName, "-F", "#{client_tty}"}
	}
	output, err := f.runCommand("tmux", args...)
	if err != nil {
		return nil, fmt.Errorf("list tmux clients: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	clients := make([]string, 0, len(lines))
	for _, line := range lines {
		if c := strings.TrimSpace(line); c != "" {
			clients = append(clients, c)
		}
	}
	return clients, nil
}

// activePaneTTY returns the TTY of the currently active (focused) tmux pane.
// Returns empty string if tmux is not running or the query fails.
func activePaneTTY(runner focusRunner) (string, error) {
	output, err := runner("tmux", "display-message", "-p", "#{pane_tty}")
	if err != nil {
		return "", fmt.Errorf("active pane tty: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
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

