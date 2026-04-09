package socket

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadClaudeSessionMetadataReadsSubagents(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("HOME", baseDir)

	cwd := "/home/katsuma/src/github.com/ka2n/way-island"
	projectDir := filepath.Join(baseDir, ".claude", "projects", encodeClaudeProjectPath(cwd))
	subagentsDir := filepath.Join(projectDir, "session-1", "subagents")
	if err := os.MkdirAll(subagentsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	parentLog := filepath.Join(projectDir, "session-1.jsonl")
	parentContent := `{"toolUseResult":{"agentId":"a9a5daa5eb7af9fd8"}}` + "\n"
	if err := os.WriteFile(parentLog, []byte(parentContent), 0o644); err != nil {
		t.Fatalf("write parent log: %v", err)
	}

	metaPath := filepath.Join(subagentsDir, "agent-a9a5daa5eb7af9fd8.meta.json")
	if err := os.WriteFile(metaPath, []byte(`{"agentType":"general-purpose","description":"Claude latest version research"}`), 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
	subagentLog := filepath.Join(subagentsDir, "agent-a9a5daa5eb7af9fd8.jsonl")
	subagentContent := "" +
		`{"type":"assistant","message":{"stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash"}]}}` + "\n"
	if err := os.WriteFile(subagentLog, []byte(subagentContent), 0o644); err != nil {
		t.Fatalf("write subagent log: %v", err)
	}

	metadata, ok := readClaudeSessionMetadata("session-1", map[string]any{"cwd": cwd})
	if !ok {
		t.Fatalf("expected metadata to be found")
	}
	if len(metadata.Subagents) != 1 {
		t.Fatalf("subagents len = %d, want 1", len(metadata.Subagents))
	}
	if metadata.Subagents[0].ID != "a9a5daa5eb7af9fd8" {
		t.Fatalf("subagent id = %q", metadata.Subagents[0].ID)
	}
	if metadata.Subagents[0].Description != "Claude latest version research" {
		t.Fatalf("subagent description = %q", metadata.Subagents[0].Description)
	}
	if metadata.Subagents[0].State != string(SessionStateToolRunning) {
		t.Fatalf("subagent state = %q, want %q", metadata.Subagents[0].State, SessionStateToolRunning)
	}
}

func TestReadClaudeLastAssistantMessage(t *testing.T) {
	t.Parallel()

	t.Run("returns last end_turn text", func(t *testing.T) {
		t.Parallel()
		f, err := os.CreateTemp(t.TempDir(), "*.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		content := "" +
			`{"type":"assistant","message":{"stop_reason":"end_turn","content":[{"type":"text","text":"first response"}]}}` + "\n" +
			`{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]}}` + "\n" +
			`{"type":"assistant","message":{"stop_reason":"end_turn","content":[{"type":"text","text":"second response"}]}}` + "\n"
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()

		got, ok := readClaudeLastAssistantMessage(map[string]any{"transcript_path": f.Name()})
		if !ok {
			t.Fatal("expected message to be found")
		}
		if got != "second response" {
			t.Fatalf("got %q, want %q", got, "second response")
		}
	})

	t.Run("ignores tool_use entries", func(t *testing.T) {
		t.Parallel()
		f, err := os.CreateTemp(t.TempDir(), "*.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		content := "" +
			`{"type":"assistant","message":{"stop_reason":"end_turn","content":[{"type":"text","text":"the response"}]}}` + "\n" +
			`{"type":"assistant","message":{"stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash"}]}}` + "\n"
		if _, err := f.WriteString(content); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()

		got, ok := readClaudeLastAssistantMessage(map[string]any{"transcript_path": f.Name()})
		if !ok {
			t.Fatal("expected message to be found")
		}
		if got != "the response" {
			t.Fatalf("got %q, want %q", got, "the response")
		}
	})

	t.Run("returns false for empty transcript", func(t *testing.T) {
		t.Parallel()
		f, err := os.CreateTemp(t.TempDir(), "*.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()

		_, ok := readClaudeLastAssistantMessage(map[string]any{"transcript_path": f.Name()})
		if ok {
			t.Fatal("expected no message")
		}
	})
}

func TestSessionManagerReadsLastAssistantMessageOnStop(t *testing.T) {
	t.Parallel()

	transcriptFile, err := os.CreateTemp(t.TempDir(), "*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	content := `{"type":"assistant","message":{"stop_reason":"end_turn","content":[{"type":"text","text":"here is my answer"}]}}` + "\n"
	if _, err := transcriptFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = transcriptFile.Close()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.Start(context.Background(), DefaultSessionTimeout)

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "session_start",
		Data: map[string]any{
			"_hook_source": "claude",
		},
	})
	waitForSessionUpdate(t, manager.Updates())

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     string(SessionStateIdle),
		Data: map[string]any{
			"_hook_source":    "claude",
			"transcript_path": transcriptFile.Name(),
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.LastAssistantMessage != "here is my answer" {
		t.Fatalf("LastAssistantMessage = %q, want %q", update.Session.LastAssistantMessage, "here is my answer")
	}
}

func TestDeriveClaudeSubagentState(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "assistant tool use means running tool",
			line: `{"type":"assistant","message":{"stop_reason":"tool_use","content":[{"type":"tool_use","name":"Bash"}]}}`,
			want: string(SessionStateToolRunning),
		},
		{
			name: "assistant end turn means idle",
			line: `{"type":"assistant","message":{"stop_reason":"end_turn","content":[{"type":"text","text":"done"}]}}`,
			want: string(SessionStateIdle),
		},
		{
			name: "tool result means working",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","content":"ok","is_error":false}]}}`,
			want: string(SessionStateWorking),
		},
		{
			name: "rejected tool result means idle",
			line: `{"type":"user","toolUseResult":"User rejected tool use"}`,
			want: string(SessionStateIdle),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := deriveClaudeSubagentState([]byte(tc.line))
			if !ok {
				t.Fatalf("expected state")
			}
			if got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}
