package socket

import (
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

	metadata, ok := readClaudeSessionMetadata("session-1", cwd)
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
