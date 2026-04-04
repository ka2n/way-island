package socket

import (
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

	metadata, ok := readCodexSessionMetadata(sessionID)
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
