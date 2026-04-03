package socket

import (
	"testing"
	"time"
)

func TestSessionManagerAddsSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager(DefaultSessionTimeout)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateUpsert {
		t.Fatalf("unexpected update type: %q", update.Type)
	}
	if update.Session.ID != "session-1" {
		t.Fatalf("unexpected session ID: %q", update.Session.ID)
	}
	if update.Session.State != SessionStateWorking {
		t.Fatalf("unexpected session state: %q", update.Session.State)
	}

	sessions := manager.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if session := sessions["session-1"]; session.LastEventAt != now {
		t.Fatalf("unexpected last event time: %v", session.LastEventAt)
	}
}

func TestSessionManagerUpdatesExistingSession(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	now := base
	manager := NewSessionManager(DefaultSessionTimeout)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	now = base.Add(5 * time.Second)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "tool_running",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateToolRunning {
		t.Fatalf("unexpected session state: %q", update.Session.State)
	}
	if update.Session.LastEventAt != now {
		t.Fatalf("unexpected update timestamp: %v", update.Session.LastEventAt)
	}

	sessions := manager.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if session := sessions["session-1"]; session.State != SessionStateToolRunning {
		t.Fatalf("unexpected stored state: %q", session.State)
	}
}

func TestSessionManagerTimesOutInactiveSessions(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	now := base
	manager := NewSessionManager(30 * time.Second)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "waiting",
		Data:      map[string]any{},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	now = base.Add(31 * time.Second)
	manager.pruneExpired(now)

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateTimeout {
		t.Fatalf("unexpected update type: %q", update.Type)
	}
	if update.Session.ID != "session-1" {
		t.Fatalf("unexpected timed-out session: %q", update.Session.ID)
	}

	sessions := manager.Sessions()
	if len(sessions) != 0 {
		t.Fatalf("expected timed-out session to be removed, got %d sessions", len(sessions))
	}
}

func TestSessionManagerMapsClaudeHookEvents(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "session_start",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateWorking {
		t.Fatalf("unexpected session state: %q", update.Session.State)
	}

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "response",
		Data:      map[string]any{},
	})

	update = waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateWaiting {
		t.Fatalf("unexpected response state: %q", update.Session.State)
	}
}

func TestSessionManagerRemovesSessionOnSessionEnd(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "session_end",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateTimeout {
		t.Fatalf("unexpected update type: %q", update.Type)
	}

	sessions := manager.Sessions()
	if len(sessions) != 0 {
		t.Fatalf("expected session to be removed, got %d sessions", len(sessions))
	}
}

func waitForSessionUpdate(t *testing.T, updates <-chan SessionUpdate) SessionUpdate {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session update")
		return SessionUpdate{}
	}
}
