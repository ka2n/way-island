package main

import (
	"strings"
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestOverlayModelPayloadSortsByLastEventAtDescending(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-old",
			DisplayName: "project-old",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(10, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-new",
			DisplayName: "project-new",
			State:       socket.SessionStateToolRunning,
			LastEventAt: time.Unix(20, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	// Most recent session (session-new, LastEventAt=20) should be first
	// base64("project-new") = "cHJvamVjdC1uZXc="
	if !strings.HasPrefix(lines[0], "cHJvamVjdC1uZXc=\t") {
		t.Fatalf("expected project-new first (most recent), got %q", lines[0])
	}

	// base64("project-old") = "cHJvamVjdC1vbGQ="
	if !strings.HasPrefix(lines[1], "cHJvamVjdC1vbGQ=\t") {
		t.Fatalf("expected project-old second, got %q", lines[1])
	}
}

func TestOverlayModelPayloadTiebreaksByID(t *testing.T) {
	model := newOverlayModel()

	sameTime := time.Unix(10, 0)
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-b",
			DisplayName: "project-b",
			State:       socket.SessionStateWorking,
			LastEventAt: sameTime,
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-a",
			DisplayName: "project-a",
			State:       socket.SessionStateToolRunning,
			LastEventAt: sameTime,
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(lines))
	}

	// Same LastEventAt → sort by ID ascending
	// base64("project-a") = "cHJvamVjdC1h"
	if !strings.HasPrefix(lines[0], "cHJvamVjdC1h\t") {
		t.Fatalf("expected project-a first (ID tiebreak), got %q", lines[0])
	}
}

func TestOverlayModelRemovesTimedOutSession(t *testing.T) {
	model := newOverlayModel()

	session := socket.Session{
		ID:          "session-1",
		State:       socket.SessionStateWaiting,
		LastEventAt: time.Unix(10, 0),
	}

	model.Apply(socket.SessionUpdate{
		Type:    socket.SessionUpdateUpsert,
		Session: session,
	})
	model.Apply(socket.SessionUpdate{
		Type:    socket.SessionUpdateTimeout,
		Session: session,
	})

	if payload := model.Payload(); payload != "" {
		t.Fatalf("expected empty payload after timeout, got %q", payload)
	}
}

func TestOverlayModelIgnoresOlderUpsert(t *testing.T) {
	model := newOverlayModel()

	// Apply a recent hook update
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			State:       socket.SessionStateIdle,
			LastEventAt: time.Unix(20, 0),
		},
	})

	// Apply an older JSONL update — should be ignored
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(10, 0),
		},
	})

	sessions := model.Sessions()
	if sessions["session-1"].State != socket.SessionStateIdle {
		t.Errorf("state = %q, want %q (older update should be ignored)", sessions["session-1"].State, socket.SessionStateIdle)
	}
}

func TestOverlayModelPayloadFallsBackToIDPrefix(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "abcdefghijklmnop",
			DisplayName: "",
			State:       socket.SessionStateIdle,
			LastEventAt: time.Unix(10, 0),
		},
	})

	payload := model.Payload()
	// base64("abcdefgh") = "YWJjZGVmZ2g="
	if !strings.Contains(payload, "YWJjZGVmZ2g=") {
		t.Fatalf("expected ID prefix fallback, got %q", payload)
	}
}
