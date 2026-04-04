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
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(20, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-new" || first[1] != "project-new" || first[2] != string(socket.SessionStateWorking) || first[3] != "" {
		t.Fatalf("unexpected first payload line: %#v", first)
	}

	second := decodePayloadFields(t, lines[1])
	if second[0] != "session-old" || second[1] != "project-old" || second[2] != string(socket.SessionStateWorking) || second[3] != "" {
		t.Fatalf("unexpected second payload line: %#v", second)
	}
}

func TestOverlayModelPayloadPrioritizesWaitingSessions(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-working",
			DisplayName: "project-working",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(20, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-waiting",
			DisplayName: "project-waiting",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(10, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-waiting" || first[2] != string(socket.SessionStateWaiting) {
		t.Fatalf("expected waiting session first, got %#v", first)
	}
}

func TestOverlayModelPayloadPrioritizesWorkingAheadOfOtherStates(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-idle",
			DisplayName: "project-idle",
			State:       socket.SessionStateIdle,
			LastEventAt: time.Unix(20, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-working",
			DisplayName: "project-working",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(10, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-working" || first[2] != string(socket.SessionStateWorking) {
		t.Fatalf("expected working session before idle, got %#v", first)
	}
}

func TestOverlayModelPayloadSortsWaitingSessionsByLastEventAtDescending(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-waiting-old",
			DisplayName: "project-waiting-old",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(10, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-waiting-new",
			DisplayName: "project-waiting-new",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(20, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-waiting-new" || first[2] != string(socket.SessionStateWaiting) {
		t.Fatalf("expected newer waiting session first, got %#v", first)
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
			State:       socket.SessionStateWorking,
			LastEventAt: sameTime,
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
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

func TestOverlayModelPayloadIncludesCurrentAction(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:            "session-1",
			DisplayName:   "project",
			State:         socket.SessionStateToolRunning,
			CurrentAction: "Bash: go test ./...",
			LastEventAt:   time.Unix(10, 0),
		},
	})

	fields := decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	if fields[3] != "Bash: go test ./..." {
		t.Fatalf("expected current action in payload, got %#v", fields)
	}
}

func TestOverlayModelPayloadIncludesLastUserMessage(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:              "session-1",
			DisplayName:     "project",
			State:           socket.SessionStateWorking,
			LastUserMessage: "show me the session detail snippet",
			LastEventAt:     time.Unix(10, 0),
		},
	})

	fields := decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	if fields[4] != "show me the session detail snippet" {
		t.Fatalf("expected last user message in payload, got %#v", fields)
	}
}

func TestOverlayModelPayloadIncludesHookSource(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWorking,
			HookSource:  "codex",
			LastEventAt: time.Unix(10, 0),
		},
	})

	fields := decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	if fields[8] != "codex" {
		t.Fatalf("expected hook source in payload, got %#v", fields)
	}
}

func TestOverlayModelPayloadTruncatesLastUserMessageByRune(t *testing.T) {
	model := newOverlayModel()

	message := strings.Repeat("あ", maxLastUserMessageRunes+5) + "🙂"
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:              "session-1",
			DisplayName:     "project",
			State:           socket.SessionStateWorking,
			LastUserMessage: message,
			LastEventAt:     time.Unix(10, 0),
		},
	})

	fields := decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	got := fields[4]
	want := strings.Repeat("あ", maxLastUserMessageRunes) + "..."
	if got != want {
		t.Fatalf("expected truncated last user message %q, got %q", want, got)
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
