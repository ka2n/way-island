package main

import (
	"strings"
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestOverlayModelPayloadSortsSessionsAndEncodesSnapshot(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-b",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(20, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-a",
			State:       socket.SessionStateToolRunning,
			LastEventAt: time.Unix(10, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	if !strings.HasPrefix(lines[0], "c2Vzc2lvbi1h\t") {
		t.Fatalf("expected sorted session-a first, got %q", lines[0])
	}

	if !strings.HasSuffix(lines[0], "\tdG9vbF9ydW5uaW5n") {
		t.Fatalf("expected tool_running state for session-a, got %q", lines[0])
	}

	if !strings.HasPrefix(lines[1], "c2Vzc2lvbi1i\t") {
		t.Fatalf("expected session-b second, got %q", lines[1])
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
