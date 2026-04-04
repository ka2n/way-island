package socket

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SubagentSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
}

type claudeSessionMetadata struct {
	Subagents []SubagentSummary
}

var readClaudeSessionMetadataFunc = readClaudeSessionMetadata

func readClaudeSessionMetadata(sessionID string, cwd string) (claudeSessionMetadata, bool) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(cwd) == "" {
		return claudeSessionMetadata{}, false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return claudeSessionMetadata{}, false
	}

	projectDir := filepath.Join(homeDir, ".claude", "projects", encodeClaudeProjectPath(cwd))
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	agentIDs, ok := readClaudeSubagentIDs(sessionPath)
	if !ok || len(agentIDs) == 0 {
		return claudeSessionMetadata{}, false
	}

	subagentsDir := filepath.Join(projectDir, sessionID, "subagents")
	subagents := make([]SubagentSummary, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		transcriptPath := filepath.Join(subagentsDir, "agent-"+agentID+".jsonl")
		subagent := SubagentSummary{
			ID:    agentID,
			Title: "agent-" + agentID,
			State: string(SessionStateIdle),
		}
		if state, ok := readClaudeSubagentState(transcriptPath); ok {
			subagent.State = state
		}
		if description := readClaudeSubagentDescription(filepath.Join(subagentsDir, "agent-"+agentID+".meta.json")); description != "" {
			subagent.Description = description
		}
		subagents = append(subagents, subagent)
	}
	return claudeSessionMetadata{Subagents: subagents}, len(subagents) > 0
}

func encodeClaudeProjectPath(cwd string) string {
	return strings.ReplaceAll(strings.TrimSpace(cwd), string(filepath.Separator), "-")
}

func readClaudeSubagentIDs(path string) ([]string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	seen := map[string]struct{}{}
	var agentIDs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry struct {
			ToolUseResult *struct {
				AgentID string `json:"agentId"`
			} `json:"toolUseResult"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.ToolUseResult == nil || strings.TrimSpace(entry.ToolUseResult.AgentID) == "" {
			continue
		}
		agentID := strings.TrimSpace(entry.ToolUseResult.AgentID)
		if _, ok := seen[agentID]; ok {
			continue
		}
		seen[agentID] = struct{}{}
		agentIDs = append(agentIDs, agentID)
	}
	if len(agentIDs) == 0 {
		return nil, false
	}
	sort.Strings(agentIDs)
	return agentIDs, true
}

func readClaudeSubagentDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var meta struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.Description)
}

func readClaudeSubagentState(path string) (string, bool) {
	file, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer file.Close()

	state := string(SessionStateIdle)
	found := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		nextState, ok := deriveClaudeSubagentState(scanner.Bytes())
		if !ok {
			continue
		}
		state = nextState
		found = true
	}
	if !found {
		return "", false
	}
	return state, true
}

func deriveClaudeSubagentState(line []byte) (string, bool) {
	var entry struct {
		Type          string `json:"type"`
		ToolUseResult string `json:"toolUseResult"`
		Message       *struct {
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				IsError bool   `json:"is_error"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return "", false
	}

	if strings.Contains(strings.ToLower(strings.TrimSpace(entry.ToolUseResult)), "rejected") {
		return string(SessionStateIdle), true
	}
	if entry.Message == nil {
		return "", false
	}

	switch strings.TrimSpace(entry.Message.StopReason) {
	case "tool_use":
		return string(SessionStateToolRunning), true
	case "end_turn":
		return string(SessionStateIdle), true
	}

	for _, content := range entry.Message.Content {
		switch {
		case content.Type == "tool_result" && content.IsError:
			return string(SessionStateIdle), true
		case content.Type == "tool_result":
			return string(SessionStateWorking), true
		case content.Type == "text" && strings.Contains(content.Text, "[Request interrupted by user for tool use]"):
			return string(SessionStateIdle), true
		}
	}

	if entry.Type == "assistant" {
		return string(SessionStateWorking), true
	}
	return "", false
}
