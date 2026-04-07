package main

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestOverlayModelPayloadUsesInsertionOrder(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-first",
			DisplayName: "project-first",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(10, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-second",
			DisplayName: "project-second",
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
	if first[0] != "session-first" {
		t.Fatalf("expected insertion-order first session, got %#v", first)
	}

	second := decodePayloadFields(t, lines[1])
	if second[0] != "session-second" {
		t.Fatalf("expected insertion-order second session, got %#v", second)
	}
}

func TestOverlayModelPayloadNoReorderOnStateChange(t *testing.T) {
	model := newOverlayModel()

	// session-working added first
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-working",
			DisplayName: "project-working",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(10, 0),
		},
	})
	// session-waiting added second
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-waiting",
			DisplayName: "project-waiting",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(20, 0),
		},
	})

	// State change on working session — should NOT cause reorder
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-working",
			DisplayName: "project-working",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(30, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 sessions in payload, got %d", len(lines))
	}

	first := decodePayloadFields(t, lines[0])
	if first[0] != "session-working" {
		t.Fatalf("expected insertion-order preserved after state change, got %#v", first)
	}
}

func TestOverlayModelPayloadNewSessionAppendsToEnd(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-a",
			DisplayName: "project-a",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(10, 0),
		},
	})
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-b",
			DisplayName: "project-b",
			State:       socket.SessionStateWaiting,
			LastEventAt: time.Unix(20, 0),
		},
	})
	// New session added after existing ones — should appear last
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-c",
			DisplayName: "project-c",
			State:       socket.SessionStateWorking,
			LastEventAt: time.Unix(5, 0),
		},
	})

	payload := model.Payload()
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 sessions in payload, got %d", len(lines))
	}

	last := decodePayloadFields(t, lines[2])
	if last[0] != "session-c" {
		t.Fatalf("expected new session appended to end, got %#v", last)
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

func TestOverlayModelPayloadIncludesSuppressedFlag(t *testing.T) {
	model := newOverlayModel()

	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			DisplayName: "project",
			State:       socket.SessionStateWorking,
			AgentTTY:    "/dev/pts/5",
			LastEventAt: time.Unix(10, 0),
		},
	})

	// Not suppressed initially
	fields := decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	if len(fields) < 11 {
		t.Fatalf("expected 11 fields, got %d", len(fields))
	}
	if fields[10] != "0" {
		t.Fatalf("IsSuppressed = %q, want %q", fields[10], "0")
	}

	// Mark as suppressed
	model.SetSuppressed("session-1", true)
	fields = decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	if fields[10] != "1" {
		t.Fatalf("IsSuppressed = %q, want %q after SetSuppressed(true)", fields[10], "1")
	}

	// Unmark
	model.SetSuppressed("session-1", false)
	fields = decodePayloadFields(t, strings.TrimSuffix(model.Payload(), "\n"))
	if fields[10] != "0" {
		t.Fatalf("IsSuppressed = %q, want %q after SetSuppressed(false)", fields[10], "0")
	}
}

func TestCountSessionsByClassSkipsSuppressed(t *testing.T) {
	t.Parallel()

	sessions := []payloadSession{
		{ID: "s1", State: "waiting", IsSuppressed: false},
		{ID: "s2", State: "waiting", IsSuppressed: true},  // suppressed → skip
		{ID: "s3", State: "working", IsSuppressed: false},
		{ID: "s4", State: "working", IsSuppressed: true},  // suppressed → skip
	}

	waitingCount := countSessionsByClass(sessions, "waiting")
	if waitingCount != 1 {
		t.Fatalf("waitingCount = %d, want 1 (suppressed session excluded)", waitingCount)
	}
	workingCount := countSessionsByClass(sessions, "working")
	if workingCount != 1 {
		t.Fatalf("workingCount = %d, want 1 (suppressed session excluded)", workingCount)
	}
}

func TestActivePaneTTYSmartSuppress(t *testing.T) {
	t.Parallel()

	model := newOverlayModel()
	model.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:          "session-1",
			State:       socket.SessionStateWaiting,
			AgentTTY:    "/dev/pts/42",
			LastEventAt: time.Unix(10, 0),
		},
	})

	// Simulate active pane matching
	model.SetSuppressed("session-1", true)

	// Build pill: WaitingCount should be 0 (suppressed)
	sessions := parsePayloadSessions(model.Payload())
	pill := buildPillViewModel(sessions)
	if pill.WaitingCount != 0 {
		t.Fatalf("WaitingCount = %d, want 0 (session is suppressed)", pill.WaitingCount)
	}
	// BadgeCount still includes it
	if pill.BadgeCount != 1 {
		t.Fatalf("BadgeCount = %d, want 1", pill.BadgeCount)
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
