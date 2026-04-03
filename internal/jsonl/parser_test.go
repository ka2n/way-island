package jsonl

import (
	"testing"

	"github.com/ka2n/way-island/internal/socket"
)

func ptr(s string) *string { return &s }

func TestParseEntry(t *testing.T) {
	t.Parallel()

	line := []byte(`{"type":"user","sessionId":"abc-123","cwd":"/home/user/project","timestamp":"2026-04-03T05:16:10.560Z","message":{"role":"user","content":"hello"}}`)

	entry, err := ParseEntry(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.Type != "user" {
		t.Errorf("type = %q, want %q", entry.Type, "user")
	}
	if entry.SessionID != "abc-123" {
		t.Errorf("sessionId = %q, want %q", entry.SessionID, "abc-123")
	}
	if entry.CWD != "/home/user/project" {
		t.Errorf("cwd = %q, want %q", entry.CWD, "/home/user/project")
	}
}

func TestParseEntry_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseEntry([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDetermineState_UserMessage(t *testing.T) {
	t.Parallel()

	entry := Entry{
		Type:    "user",
		Message: &Message{Role: "user", Content: []byte(`"hello"`)},
	}

	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateWorking {
		t.Errorf("state = %q, want %q", state, socket.SessionStateWorking)
	}
}

func TestDetermineState_UserToolResult(t *testing.T) {
	t.Parallel()

	entry := Entry{
		Type: "user",
		Message: &Message{
			Role:    "user",
			Content: []byte(`[{"type":"tool_result","tool_use_id":"toolu_123","content":"file contents"}]`),
		},
	}

	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateWorking {
		t.Errorf("state = %q, want %q", state, socket.SessionStateWorking)
	}
}

func TestDetermineState_AssistantToolUse(t *testing.T) {
	t.Parallel()

	entry := Entry{
		Type: "assistant",
		Message: &Message{
			Role:    "assistant",
			Content: []byte(`[{"type":"text","text":"reading file"},{"type":"tool_use","name":"Read","id":"toolu_123"}]`),
		},
	}

	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateToolRunning {
		t.Errorf("state = %q, want %q", state, socket.SessionStateToolRunning)
	}
}

func TestDetermineState_AssistantEndTurn(t *testing.T) {
	t.Parallel()

	entry := Entry{
		Type: "assistant",
		Message: &Message{
			Role:       "assistant",
			Content:    []byte(`[{"type":"text","text":"done"}]`),
			StopReason: ptr("end_turn"),
		},
	}

	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateIdle {
		t.Errorf("state = %q, want %q", state, socket.SessionStateIdle)
	}
}

func TestDetermineState_AssistantWorking(t *testing.T) {
	t.Parallel()

	entry := Entry{
		Type: "assistant",
		Message: &Message{
			Role:    "assistant",
			Content: []byte(`[{"type":"text","text":"thinking..."}]`),
		},
	}

	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateWorking {
		t.Errorf("state = %q, want %q", state, socket.SessionStateWorking)
	}
}

func TestDetermineState_SystemStopHookSummary(t *testing.T) {
	t.Parallel()

	entry := Entry{Type: "system", Subtype: "stop_hook_summary"}
	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateIdle {
		t.Errorf("state = %q, want %q", state, socket.SessionStateIdle)
	}
}

func TestDetermineState_SystemTurnDuration(t *testing.T) {
	t.Parallel()

	entry := Entry{Type: "system", Subtype: "turn_duration"}
	state, ok := DetermineState(entry)
	if !ok {
		t.Fatal("expected ok")
	}
	if state != socket.SessionStateIdle {
		t.Errorf("state = %q, want %q", state, socket.SessionStateIdle)
	}
}

func TestDetermineState_IgnoredTypes(t *testing.T) {
	t.Parallel()

	for _, typ := range []string{"file-history-snapshot", "attachment"} {
		_, ok := DetermineState(Entry{Type: typ})
		if ok {
			t.Errorf("type %q should not produce a state", typ)
		}
	}

	// system without relevant subtype should be ignored
	_, ok := DetermineState(Entry{Type: "system", Subtype: "local_command"})
	if ok {
		t.Error("system/local_command should not produce a state")
	}
}

func TestDetermineState_NilMessage(t *testing.T) {
	t.Parallel()

	_, ok := DetermineState(Entry{Type: "assistant"})
	if ok {
		t.Error("nil message should not produce a state")
	}
}

func TestExtractToolName(t *testing.T) {
	t.Parallel()

	content := []byte(`[{"type":"text","text":"reading"},{"type":"tool_use","name":"Read"},{"type":"tool_use","name":"Bash"}]`)
	name := ExtractToolName(content)
	if name != "Bash" {
		t.Errorf("tool name = %q, want %q", name, "Bash")
	}
}

func TestExtractToolName_NoToolUse(t *testing.T) {
	t.Parallel()

	content := []byte(`[{"type":"text","text":"hello"}]`)
	name := ExtractToolName(content)
	if name != "" {
		t.Errorf("tool name = %q, want empty", name)
	}
}

func TestResolveDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cwd  string
		want string
	}{
		{"/home/user/projects/my-app", "my-app"},
		{"/", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := ResolveDisplayName(Entry{CWD: tt.cwd})
		if got != tt.want {
			t.Errorf("ResolveDisplayName(cwd=%q) = %q, want %q", tt.cwd, got, tt.want)
		}
	}
}

func TestSessionIDFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want string
	}{
		{"/home/user/.claude/projects/foo/abc-123.jsonl", "abc-123"},
		{"abc-123.jsonl", "abc-123"},
		{"abc-123", "abc-123"},
	}

	for _, tt := range tests {
		got := SessionIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("SessionIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
