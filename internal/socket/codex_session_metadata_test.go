package socket

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadCodexSessionMetadataReadsSubagentFields(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("HOME", baseDir)

	sessionID := "child-session"
	path := filepath.Join(baseDir, ".codex", "sessions", "2026", "04", "04", "rollout-2026-04-04T17-55-00-"+sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `{"type":"session_meta","payload":{"id":"child-session","forked_from_id":"parent-session","source":{"subagent":{"thread_spawn":{"parent_thread_id":"parent-session","depth":1}}},"agent_nickname":"Harvey"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	metadata, ok := readCodexSessionMetadata(sessionID, nil)
	if !ok {
		t.Fatalf("expected metadata to be found")
	}
	if !metadata.IsSubagent {
		t.Fatalf("IsSubagent = false, want true")
	}
	if metadata.ParentSessionID != "parent-session" {
		t.Fatalf("ParentSessionID = %q, want %q", metadata.ParentSessionID, "parent-session")
	}
	if metadata.AgentNickname != "Harvey" {
		t.Fatalf("AgentNickname = %q, want %q", metadata.AgentNickname, "Harvey")
	}
}

func TestReadCodexLastAssistantMessage(t *testing.T) {
	t.Parallel()

	t.Run("returns message text", func(t *testing.T) {
		t.Parallel()
		got, ok := readCodexLastAssistantMessage(map[string]any{"last_assistant_message": "hello from codex"})
		if !ok {
			t.Fatal("expected message to be found")
		}
		if got != "hello from codex" {
			t.Fatalf("got %q, want %q", got, "hello from codex")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Parallel()
		got, ok := readCodexLastAssistantMessage(map[string]any{"last_assistant_message": "  trimmed  "})
		if !ok {
			t.Fatal("expected message")
		}
		if got != "trimmed" {
			t.Fatalf("got %q, want %q", got, "trimmed")
		}
	})

	t.Run("returns false when absent", func(t *testing.T) {
		t.Parallel()
		_, ok := readCodexLastAssistantMessage(map[string]any{})
		if ok {
			t.Fatal("expected no message")
		}
	})
}

func TestSessionManagerReadsLastAssistantMessageOnStopForCodex(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.Start(context.Background(), DefaultSessionTimeout)

	manager.HandleMessage(Message{
		SessionID: "session-cx",
		Event:     "session_start",
		Data: map[string]any{
			"_hook_source": "codex",
		},
	})
	waitForSessionUpdate(t, manager.Updates())

	// Codex Stop hook provides last_assistant_message directly in the payload.
	manager.HandleMessage(Message{
		SessionID: "session-cx",
		Event:     string(SessionStateIdle),
		Data: map[string]any{
			"_hook_source":           "codex",
			"last_assistant_message": "codex answer",
			"transcript_path":        nil, // transcript_path can be null for Codex
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.LastAssistantMessage != "codex answer" {
		t.Fatalf("LastAssistantMessage = %q, want %q", update.Session.LastAssistantMessage, "codex answer")
	}
}
