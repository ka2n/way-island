package main

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestCollectAncestorPIDs(t *testing.T) {
	parents := map[int]int{
		420: 210,
		210: 100,
		100: 1,
	}

	ancestors, err := collectAncestorPIDs(420, func(pid int) (int, error) {
		parent, ok := parents[pid]
		if !ok {
			return 0, errors.New("unknown pid")
		}
		return parent, nil
	})
	if err != nil {
		t.Fatalf("collectAncestorPIDs: %v", err)
	}

	want := map[int]int{420: 0, 210: 1, 100: 2}
	if !reflect.DeepEqual(ancestors, want) {
		t.Fatalf("ancestors = %#v, want %#v", ancestors, want)
	}
}

func TestSessionFocuserResolvesNearestTmuxPane(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:                     "session-1",
			DisplayName:            "repo",
			State:                  socket.SessionStateWorking,
			LastEventAt:            time.Unix(10, 0),
			AgentPID:               420,
			AgentPIDNamespaceInode: 9999,
			AgentStartTimeTicks:    1234,
		},
	})

	var commands [][]string
	var focused []string
	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			commands = append(commands, call)
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("100\tmain\teditor\t%7\t/dev/pts/20\n210\tmain\tlogs\t%8\t/dev/pts/21\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				return []byte("/dev/pts/11\n"), nil
			}
			return nil, nil
		},
		focusWindow: func(appID string, title string) error {
			focused = []string{appID, title}
			return nil
		},
		writeTTY: func(path string, data []byte) error { return nil },
		parentPID: func(pid int) (int, error) {
			switch pid {
			case 9210:
				return 210, nil
			case 210:
				return 100, nil
			case 100:
				return 1, nil
			default:
				return 0, errors.New("unknown pid")
			}
		},
		sleep:                 func(time.Duration) {},
		readCurrentPIDNSInode: func() (uint64, error) { return 1111, nil },
		readPIDNSInode: func(pid int) (uint64, error) {
			if pid == 9210 {
				return 9999, nil
			}
			return 1111, nil
		},
		readStartTimeTicks: func(pid int) (uint64, error) {
			switch pid {
			case 9210:
				return 1234, nil
			case 420:
				return 55, nil
			default:
				return 0, errors.New("unknown pid")
			}
		},
		listProcPIDs: func() ([]int, error) { return []int{9210}, nil },
		readNSPIDs:   func(pid int) ([]int, error) { return []int{9210, 420}, nil },
	}

	if err := focuser.Focus("session-1"); err != nil {
		t.Fatalf("Focus: %v", err)
	}

	want := [][]string{
		{"tmux", "list-panes", "-a", "-F", "#{pane_pid}\t#{session_name}\t#{window_name}\t#{pane_id}\t#{pane_tty}"},
		{"tmux", "list-clients", "-t", "main", "-F", "#{client_tty}"},
		{"tmux", "switch-client", "-c", "/dev/pts/11", "-t", "%8"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	if wantFocus := []string{"Alacritty", "main:logs"}; !reflect.DeepEqual(focused, wantFocus) {
		t.Fatalf("focused = %#v, want %#v", focused, wantFocus)
	}
}

func TestSessionFocuserReturnsErrorWithoutAttachedTmuxClient(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:                     "session-1",
			LastEventAt:            time.Unix(10, 0),
			AgentPID:               420,
			AgentPIDNamespaceInode: 9999,
			AgentStartTimeTicks:    1234,
		},
	})

	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("210\tmain\tlogs\t%8\t/dev/pts/21\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				return []byte(""), nil
			}
			return nil, nil
		},
		writeTTY: func(path string, data []byte) error { return nil },
		parentPID: func(pid int) (int, error) {
			switch pid {
			case 420:
				return 210, nil
			case 210:
				return 1, nil
			default:
				return 0, errors.New("unknown pid")
			}
		},
		sleep:                 func(time.Duration) {},
		readCurrentPIDNSInode: func() (uint64, error) { return 1111, nil },
		readPIDNSInode:        func(pid int) (uint64, error) { return 9999, nil },
		readStartTimeTicks: func(pid int) (uint64, error) {
			if pid == 210 {
				return 1234, nil
			}
			return 0, errors.New("unknown pid")
		},
		listProcPIDs: func() ([]int, error) { return []int{210}, nil },
		readNSPIDs:   func(pid int) ([]int, error) { return []int{210, 420}, nil },
	}

	err := focuser.Focus("session-1")
	if !errors.Is(err, errNoTmuxClient) {
		t.Fatalf("Focus error = %v, want errNoTmuxClient", err)
	}
}

func TestSessionFocuserReturnsErrorWithoutAgentPID(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			LastEventAt: time.Unix(10, 0),
		},
	})

	focuser := newSessionFocuser(store)
	err := focuser.Focus("session-1")
	if !errors.Is(err, errNoAgentPID) {
		t.Fatalf("Focus error = %v, want errNoAgentPID", err)
	}
}

func TestSessionFocuserFallsBackToTTYMatch(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			LastEventAt: time.Unix(10, 0),
			AgentPID:    2,
			AgentTTY:    "/dev/pts/33",
		},
	})

	var commands [][]string
	var focused []string
	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			commands = append(commands, call)
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("210\tmain\tlogs\t%8\t/dev/pts/33\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				return []byte("/dev/pts/11\n"), nil
			}
			return nil, nil
		},
		focusWindow: func(appID string, title string) error {
			focused = []string{appID, title}
			return nil
		},
		writeTTY:              func(path string, data []byte) error { return nil },
		parentPID:             func(pid int) (int, error) { return 0, errors.New("not found") },
		sleep:                 func(time.Duration) {},
		readCurrentPIDNSInode: func() (uint64, error) { return 1111, nil },
		readPIDNSInode:        func(pid int) (uint64, error) { return 1111, errors.New("not found") },
		readStartTimeTicks:    func(pid int) (uint64, error) { return 0, errors.New("not found") },
		listProcPIDs:          func() ([]int, error) { return nil, nil },
		readNSPIDs:            func(pid int) ([]int, error) { return nil, errors.New("not found") },
	}

	if err := focuser.Focus("session-1"); err != nil {
		t.Fatalf("Focus: %v", err)
	}

	want := [][]string{
		{"tmux", "list-panes", "-a", "-F", "#{pane_pid}\t#{session_name}\t#{window_name}\t#{pane_id}\t#{pane_tty}"},
		{"tmux", "list-clients", "-t", "main", "-F", "#{client_tty}"},
		{"tmux", "switch-client", "-c", "/dev/pts/11", "-t", "%8"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	if wantFocus := []string{"Alacritty", "main:logs"}; !reflect.DeepEqual(focused, wantFocus) {
		t.Fatalf("focused = %#v, want %#v", focused, wantFocus)
	}
}

func TestSessionFocuserCapturesTmuxPaneText(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			LastEventAt: time.Unix(10, 0),
			AgentPID:    2,
			AgentTTY:    "/dev/pts/33",
		},
	})

	var commands [][]string
	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			commands = append(commands, call)
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("210\tmain\tlogs\t%8\t/dev/pts/33\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "capture-pane" {
				return []byte("Would you like to run the following command?"), nil
			}
			return nil, nil
		},
		parentPID:             func(pid int) (int, error) { return 0, errors.New("not found") },
		writeTTY:              func(path string, data []byte) error { return nil },
		sleep:                 func(time.Duration) {},
		readCurrentPIDNSInode: func() (uint64, error) { return 1111, nil },
		readPIDNSInode:        func(pid int) (uint64, error) { return 1111, errors.New("not found") },
		readStartTimeTicks:    func(pid int) (uint64, error) { return 0, errors.New("not found") },
		listProcPIDs:          func() ([]int, error) { return nil, nil },
		readNSPIDs:            func(pid int) ([]int, error) { return nil, errors.New("not found") },
	}

	output, err := focuser.capturePaneText("session-1")
	if err != nil {
		t.Fatalf("capturePaneText: %v", err)
	}
	if output != "Would you like to run the following command?" {
		t.Fatalf("output = %q", output)
	}

	want := [][]string{
		{"tmux", "list-panes", "-a", "-F", "#{pane_pid}\t#{session_name}\t#{window_name}\t#{pane_id}\t#{pane_tty}"},
		{"tmux", "capture-pane", "-p", "-t", "%8"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestSessionFocuserRetriesFocusAfterOSCRetitle(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			LastEventAt: time.Unix(10, 0),
			AgentPID:    2,
			AgentTTY:    "/dev/pts/33",
		},
	})

	var focusCalls [][]string
	var writes [][]byte
	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("210\tmain\tlogs\t%8\t/dev/pts/33\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				return []byte("/dev/pts/11\n"), nil
			}
			return nil, nil
		},
		focusWindow: func(appID string, title string) error {
			focusCalls = append(focusCalls, []string{appID, title})
			if len(focusCalls) == 1 {
				return errors.New(`wayland toplevel not found for app_id="Alacritty" title="main:logs"`)
			}
			return nil
		},
		writeTTY: func(path string, data []byte) error {
			if path != "/dev/pts/33" {
				t.Fatalf("path = %q, want %q", path, "/dev/pts/33")
			}
			writes = append(writes, append([]byte(nil), data...))
			return nil
		},
		parentPID:             func(pid int) (int, error) { return 0, errors.New("not found") },
		sleep:                 func(time.Duration) {},
		readCurrentPIDNSInode: func() (uint64, error) { return 1111, nil },
		readPIDNSInode:        func(pid int) (uint64, error) { return 1111, errors.New("not found") },
		readStartTimeTicks:    func(pid int) (uint64, error) { return 0, errors.New("not found") },
		listProcPIDs:          func() ([]int, error) { return nil, nil },
		readNSPIDs:            func(pid int) ([]int, error) { return nil, errors.New("not found") },
		focusConfig:           focusConfig{RetitleWithOSC: true},
	}

	if err := focuser.Focus("session-1"); err != nil {
		t.Fatalf("Focus: %v", err)
	}
	if len(focusCalls) != 2 {
		t.Fatalf("focusCalls = %#v, want two attempts", focusCalls)
	}
	if len(writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writes))
	}
	if got := string(writes[0]); got != "\033]0;main:logs\007\033]2;main:logs\007" {
		t.Fatalf("osc payload = %q", got)
	}
}

func TestSanitizeOSCTitleStripsControlBytes(t *testing.T) {
	got := sanitizeOSCTitle("main:\x1blogs\a\n")
	if got != "main:logs" {
		t.Fatalf("sanitizeOSCTitle = %q", got)
	}
}
