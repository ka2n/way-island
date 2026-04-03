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
			ID:          "session-1",
			DisplayName: "repo",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(10, 0),
			ClaudePID:   420,
		},
	})

	var commands [][]string
	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			commands = append(commands, call)
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("100\tmain\teditor\t%7\n210\tmain\tlogs\t%8\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				return []byte("/dev/pts/11\n"), nil
			}
			return nil, nil
		},
		parentPID: func(pid int) (int, error) {
			switch pid {
			case 420:
				return 210, nil
			case 210:
				return 100, nil
			case 100:
				return 1, nil
			default:
				return 0, errors.New("unknown pid")
			}
		},
		sleep: func(time.Duration) {},
	}

	if err := focuser.Focus("session-1"); err != nil {
		t.Fatalf("Focus: %v", err)
	}

	want := [][]string{
		{"tmux", "list-panes", "-a", "-F", "#{pane_pid}\t#{session_name}\t#{window_name}\t#{pane_id}"},
		{"tmux", "list-clients", "-t", "main", "-F", "#{client_tty}"},
		{"tmux", "switch-client", "-c", "/dev/pts/11", "-t", "%8"},
		{"wlrctl", "toplevel", "focus", "app_id:Alacritty", "title:logs"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestSessionFocuserReturnsErrorWithoutAttachedTmuxClient(t *testing.T) {
	store := newOverlayModel()
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			LastEventAt: time.Unix(10, 0),
			ClaudePID:   420,
		},
	})

	focuser := &sessionFocuser{
		store: store,
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name == "tmux" && len(args) >= 1 && args[0] == "list-panes" {
				return []byte("210\tmain\tlogs\t%8\n"), nil
			}
			if name == "tmux" && len(args) >= 1 && args[0] == "list-clients" {
				return []byte(""), nil
			}
			return nil, nil
		},
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
		sleep: func(time.Duration) {},
	}

	err := focuser.Focus("session-1")
	if !errors.Is(err, errNoTmuxClient) {
		t.Fatalf("Focus error = %v, want errNoTmuxClient", err)
	}
}

func TestSessionFocuserReturnsErrorWithoutClaudePID(t *testing.T) {
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
	if !errors.Is(err, errNoClaudePID) {
		t.Fatalf("Focus error = %v, want errNoClaudePID", err)
	}
}
