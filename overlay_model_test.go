package main

import (
	"encoding/base64"
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

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-new" || first[1] != "project-new" || first[2] != string(socket.SessionStateToolRunning) {
		t.Fatalf("unexpected first payload line: %#v", first)
	}

	second := decodePayloadFields(t, lines[1])
	if second[0] != "session-old" || second[1] != "project-old" || second[2] != string(socket.SessionStateWorking) {
		t.Fatalf("unexpected second payload line: %#v", second)
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

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-a" || first[1] != "project-a" {
		t.Fatalf("expected session-a first (ID tiebreak), got %#v", first)
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
	fields := decodePayloadFields(t, strings.TrimSpace(payload))
	if fields[1] != "abcdefgh" {
		t.Fatalf("expected ID prefix fallback, got %#v", fields)
	}
}

func decodePayloadFields(t *testing.T, line string) []string {
	t.Helper()

	parts := strings.Split(line, "\t")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		decoded, err := base64.StdEncoding.DecodeString(part)
		if err != nil {
			t.Fatalf("decode field %q: %v", part, err)
		}
		out = append(out, string(decoded))
	}
	return out
}
