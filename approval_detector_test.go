package main

import (
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestBashApprovalPipelineDetectsPrompt(t *testing.T) {
	text := `
Would you like to run the following command?
Yes, proceed (y)
Yes, and don't ask again for commands that start with go test -tags gtk4 ./...
Press enter to confirm or esc to cancel
`
	if !(bashApprovalPipeline{}).detect(text) {
		t.Fatal("expected approval prompt to be detected")
	}
	if (bashApprovalPipeline{}).detect("normal tool output\nall tests passed\n") {
		t.Fatal("unexpected approval prompt match")
	}
}

func TestApprovalPromptDetectorPromotesLongRunningToolToWaiting(t *testing.T) {
	store := newOverlayModel()
	session := socket.Session{
		ID:            "session-1",
		DisplayName:   "project",
		State:         socket.SessionStateToolRunning,
		CurrentTool:   "bash",
		CurrentAction: "Bash: go test -tags gtk4 ./...",
		HookSource:    "codex",
		LastEventAt:   time.Unix(100, 0),
	}
	store.Apply(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	updates := make(chan socket.SessionUpdate, 1)
	detector := &approvalPromptDetector{
		store: store,
		capturePane: func(sessionID string) (string, error) {
			return "Would you like to run the following command?\nYes, proceed\nPress enter to confirm or esc to cancel\n", nil
		},
		sleep:   func(time.Duration) {},
		now:     func() time.Time { return time.Unix(101, 0) },
		updates: updates,
		pipelines: map[string]toolApprovalPipeline{
			"bash": bashApprovalPipeline{},
		},
	}

	detector.Observe(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	select {
	case update := <-updates:
		if update.Session.State != socket.SessionStateWaiting {
			t.Fatalf("state = %q, want %q", update.Session.State, socket.SessionStateWaiting)
		}
		if update.Reason != "tmux:approval_prompt:bash" {
			t.Fatalf("reason = %q, want %q", update.Reason, "tmux:approval_prompt:bash")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for approval detection update")
	}
}

func TestApprovalPromptDetectorSkipsChangedSession(t *testing.T) {
	store := newOverlayModel()
	session := socket.Session{
		ID:            "session-1",
		State:         socket.SessionStateToolRunning,
		CurrentTool:   "bash",
		CurrentAction: "Bash: go test ./...",
		HookSource:    "codex",
		LastEventAt:   time.Unix(100, 0),
	}
	store.Apply(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})
	store.Apply(socket.SessionUpdate{
		Type: socket.SessionUpdateUpsert,
		Session: socket.Session{
			ID:            "session-1",
			State:         socket.SessionStateWorking,
			CurrentAction: "",
			LastEventAt:   time.Unix(101, 0),
		},
	})

	updates := make(chan socket.SessionUpdate, 1)
	detector := &approvalPromptDetector{
		store: store,
		capturePane: func(sessionID string) (string, error) {
			return "Would you like to run the following command?\nYes, proceed\n", nil
		},
		sleep:   func(time.Duration) {},
		now:     func() time.Time { return time.Unix(102, 0) },
		updates: updates,
		pipelines: map[string]toolApprovalPipeline{
			"bash": bashApprovalPipeline{},
		},
	}

	detector.Observe(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	select {
	case update := <-updates:
		t.Fatalf("unexpected update: %#v", update)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestApprovalPromptDetectorSkipsClaudeSessions(t *testing.T) {
	store := newOverlayModel()
	session := socket.Session{
		ID:            "session-1",
		State:         socket.SessionStateToolRunning,
		CurrentTool:   "bash",
		CurrentAction: "bash",
		HookSource:    "claude",
		LastEventAt:   time.Unix(100, 0),
	}
	store.Apply(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	updates := make(chan socket.SessionUpdate, 1)
	detector := &approvalPromptDetector{
		store: store,
		capturePane: func(sessionID string) (string, error) {
			return "Would you like to run the following command?\nYes, proceed\n", nil
		},
		sleep:   func(time.Duration) {},
		now:     func() time.Time { return time.Unix(101, 0) },
		updates: updates,
		pipelines: map[string]toolApprovalPipeline{
			"bash": bashApprovalPipeline{},
		},
	}

	detector.Observe(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	select {
	case update := <-updates:
		t.Fatalf("unexpected update: %#v", update)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestApprovalPromptDetectorSkipsUnsupportedTool(t *testing.T) {
	store := newOverlayModel()
	session := socket.Session{
		ID:            "session-1",
		State:         socket.SessionStateToolRunning,
		CurrentTool:   "applypatch",
		CurrentAction: "ApplyPatch",
		HookSource:    "codex",
		LastEventAt:   time.Unix(100, 0),
	}
	store.Apply(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	updates := make(chan socket.SessionUpdate, 1)
	detector := &approvalPromptDetector{
		store: store,
		capturePane: func(sessionID string) (string, error) {
			return "Would you like to run the following command?\nYes, proceed\n", nil
		},
		sleep:   func(time.Duration) {},
		now:     func() time.Time { return time.Unix(101, 0) },
		updates: updates,
		pipelines: map[string]toolApprovalPipeline{
			"bash": bashApprovalPipeline{},
		},
	}

	detector.Observe(socket.SessionUpdate{Type: socket.SessionUpdateUpsert, Session: session})

	select {
	case update := <-updates:
		t.Fatalf("unexpected update: %#v", update)
	case <-time.After(50 * time.Millisecond):
	}
}
