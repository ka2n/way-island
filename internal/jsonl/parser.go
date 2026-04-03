package jsonl

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

// Entry represents a single line in a Claude Code JSONL file.
type Entry struct {
	Type      string    `json:"type"`
	Subtype   string    `json:"subtype,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	CWD       string    `json:"cwd,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Message   *Message  `json:"message,omitempty"`
}

// Message represents the message field within a JSONL entry.
type Message struct {
	Role       string          `json:"role,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	StopReason *string         `json:"stop_reason,omitempty"`
}

// ContentBlock represents a single block within message.content array.
type ContentBlock struct {
	Type      string `json:"type"`
	Name      string `json:"name,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
}

// ParseEntry parses a single JSONL line into an Entry.
func ParseEntry(line []byte) (Entry, error) {
	var entry Entry
	if err := json.Unmarshal(line, &entry); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// DetermineState determines the session state from a JSONL entry.
func DetermineState(entry Entry) (socket.SessionState, bool) {
	switch entry.Type {
	case "assistant":
		return determineAssistantState(entry)
	case "user":
		return determineUserState(entry)
	case "system":
		return determineSystemState(entry)
	default:
		return "", false
	}
}

func determineAssistantState(entry Entry) (socket.SessionState, bool) {
	if entry.Message == nil {
		return "", false
	}

	// Check if message content contains tool_use blocks
	if hasToolUse(entry.Message.Content) {
		return socket.SessionStateToolRunning, true
	}

	// Assistant message with end_turn → idle (waiting for user input)
	if entry.Message.StopReason != nil && *entry.Message.StopReason == "end_turn" {
		return socket.SessionStateIdle, true
	}

	// Assistant message still streaming or other stop reason
	return socket.SessionStateWorking, true
}

func determineUserState(entry Entry) (socket.SessionState, bool) {
	if entry.Message == nil {
		return "", false
	}

	// User message with tool_result → working (Claude processing result)
	if hasToolResult(entry.Message.Content) {
		return socket.SessionStateWorking, true
	}

	// Regular user message → working (Claude will process)
	if entry.Message.Role == "user" {
		return socket.SessionStateWorking, true
	}

	return "", false
}

func determineSystemState(entry Entry) (socket.SessionState, bool) {
	switch entry.Subtype {
	case "stop_hook_summary", "turn_duration":
		// Turn has ended — session is idle (waiting for user input)
		return socket.SessionStateIdle, true
	default:
		return "", false
	}
}

func hasToolUse(content json.RawMessage) bool {
	return hasContentBlockType(content, "tool_use")
}

func hasToolResult(content json.RawMessage) bool {
	return hasContentBlockType(content, "tool_result")
}

func hasContentBlockType(content json.RawMessage, blockType string) bool {
	if len(content) == 0 {
		return false
	}

	var blocks []ContentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return false
	}

	for _, block := range blocks {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

// ExtractToolName returns the tool name from the last tool_use block in content, if any.
func ExtractToolName(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	var blocks []ContentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return ""
	}

	var last string
	for _, block := range blocks {
		if block.Type == "tool_use" {
			last = block.Name
		}
	}
	return last
}

// ResolveDisplayName extracts a human-readable session name from a JSONL entry.
func ResolveDisplayName(entry Entry) string {
	if entry.CWD != "" {
		if name := filepath.Base(entry.CWD); name != "" && name != "." && name != "/" {
			return name
		}
	}
	return ""
}

// SessionIDFromPath extracts a session ID from a JSONL file path.
// The file name (without extension) is the session ID.
func SessionIDFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == ".jsonl" {
		return base[:len(base)-len(ext)]
	}
	return base
}
