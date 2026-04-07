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
		{"tmux", "select-window", "-t", "%8"},
		{"tmux", "select-pane", "-t", "%8"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	if wantFocus := []string{"Alacritty", "main:logs"}; !reflect.DeepEqual(focused, wantFocus) {
		t.Fatalf("focused = %#v, want %#v", focused, wantFocus)
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
		{"tmux", "select-window", "-t", "%8"},
		{"tmux", "select-pane", "-t", "%8"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	if wantFocus := []string{"Alacritty", "main:logs"}; !reflect.DeepEqual(focused, wantFocus) {
		t.Fatalf("focused = %#v, want %#v", focused, wantFocus)
	}
}

func TestSessionFocuserSwitchesClientWhenNoClientAttached(t *testing.T) {
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
				return []byte("210\tother\tlogs\t%8\t/dev/pts/33\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				// No client on "other" session, but one on another session.
				if len(args) >= 3 && args[2] == "other" {
					return []byte(""), nil
				}
				return []byte("/dev/pts/11\n"), nil
			}
			return nil, nil
		},
		focusWindow: func(appID string, title string) error {
			focused = []string{appID, title}
			return nil
		},
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
		{"tmux", "list-clients", "-t", "other", "-F", "#{client_tty}"},
		{"tmux", "list-clients", "-F", "#{client_tty}"},
		{"tmux", "switch-client", "-c", "/dev/pts/11", "-t", "other"},
		{"tmux", "select-window", "-t", "%8"},
		{"tmux", "select-pane", "-t", "%8"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	if wantFocus := []string{"Alacritty", "other:logs"}; !reflect.DeepEqual(focused, wantFocus) {
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

func TestSessionFocuserRetriesWithTmuxSetTitles(t *testing.T) {
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
	var commands [][]string
	// Two clients: /dev/pts/11 (panel, on "main") and /dev/pts/22 (terminal, on "other").
	// The panel client satisfies ensureTmuxClientAttached but isn't visible to Wayland.
	// The retry must switch /dev/pts/22 from "other" to "main" for Wayland to find it.
	clientSessions := map[string]string{
		"/dev/pts/11": "main",
		"/dev/pts/22": "other",
	}
	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			commands = append(commands, call)
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("210\tmain\tlogs\t%8\t/dev/pts/33\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				if len(args) >= 3 && args[2] == "main" {
					return []byte("/dev/pts/11\n"), nil // panel client
				}
				return []byte("/dev/pts/11\n/dev/pts/22\n"), nil // all clients
			}
			if name == "tmux" && len(args) >= 2 && args[0] == "show-options" && args[1] == "-gv" {
				switch args[2] {
				case "set-titles":
					return []byte("off\n"), nil
				case "set-titles-string":
					return []byte("#W:#T\n"), nil
				}
			}
			if name == "tmux" && len(args) >= 2 && args[0] == "display-message" {
				// -t <client> -F #{session_name}
				for i, a := range args {
					if a == "-t" && i+1 < len(args) {
						if s, ok := clientSessions[args[i+1]]; ok {
							return []byte(s + "\n"), nil
						}
					}
				}
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "switch-client" {
				var tty, target string
				for i, a := range args {
					if a == "-c" && i+1 < len(args) {
						tty = args[i+1]
					}
					if a == "-t" && i+1 < len(args) {
						target = args[i+1]
					}
				}
				if tty != "" {
					clientSessions[tty] = target
				}
			}
			return nil, nil
		},
		focusWindow: func(appID string, title string) error {
			focusCalls = append(focusCalls, []string{appID, title})
			// Succeed only when /dev/pts/22 has been switched to "main".
			if clientSessions["/dev/pts/22"] == "main" {
				return nil
			}
			return errors.New(`wayland toplevel not found for app_id="Alacritty" title="main:logs"`)
		},
		parentPID:             func(pid int) (int, error) { return 0, errors.New("not found") },
		sleep:                 func(time.Duration) {},
		readCurrentPIDNSInode: func() (uint64, error) { return 1111, nil },
		readPIDNSInode:        func(pid int) (uint64, error) { return 1111, errors.New("not found") },
		readStartTimeTicks:    func(pid int) (uint64, error) { return 0, errors.New("not found") },
		listProcPIDs:          func() ([]int, error) { return nil, nil },
		readNSPIDs:            func(pid int) ([]int, error) { return nil, errors.New("not found") },
		focusConfig:           focusConfig{TmuxSetTitles: true},
	}

	if err := focuser.Focus("session-1"); err != nil {
		t.Fatalf("Focus: %v", err)
	}

	// Verify the retry switched /dev/pts/22 to "main" and restored it to "other".
	var switchCmds [][]string
	for _, cmd := range commands {
		if len(cmd) >= 2 && cmd[0] == "tmux" && cmd[1] == "switch-client" {
			switchCmds = append(switchCmds, cmd)
		}
	}
	wantSwitch := [][]string{
		// retry: switch /dev/pts/22 from "other" to "main"
		{"tmux", "switch-client", "-c", "/dev/pts/22", "-t", "main"},
		// restore: switch /dev/pts/22 back to "other" — NOT present because focus succeeded
	}
	if !reflect.DeepEqual(switchCmds, wantSwitch) {
		t.Fatalf("switch-client commands = %#v, want %#v", switchCmds, wantSwitch)
	}

	// Verify set-titles was enabled then restored.
	var setOptCmds [][]string
	for _, cmd := range commands {
		if len(cmd) >= 3 && cmd[0] == "tmux" && cmd[1] == "set-option" {
			setOptCmds = append(setOptCmds, cmd)
		}
	}
	wantOpt := [][]string{
		{"tmux", "set-option", "-g", "set-titles", "off"},
		{"tmux", "set-option", "-g", "set-titles-string", "#S:#W"},
		{"tmux", "set-option", "-g", "set-titles", "on"},
		{"tmux", "set-option", "-g", "set-titles", "off"},
		{"tmux", "set-option", "-g", "set-titles-string", "#W:#T"},
	}
	if !reflect.DeepEqual(setOptCmds, wantOpt) {
		t.Fatalf("set-option commands = %#v, want %#v", setOptCmds, wantOpt)
	}
}

func TestSanitizeOSCTitleStripsControlBytes(t *testing.T) {
	got := sanitizeOSCTitle("main:\x1blogs\a\n")
	if got != "main:logs" {
		t.Fatalf("sanitizeOSCTitle = %q", got)
	}
}
