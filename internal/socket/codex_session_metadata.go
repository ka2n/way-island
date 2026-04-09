package socket

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type codexSessionMetadata struct {
	ParentSessionID string
	IsSubagent      bool
	AgentNickname   string
}

var readCodexSessionMetadataFunc = readCodexSessionMetadata
var readCodexLastAssistantMessageFunc = readCodexLastAssistantMessage

func readCodexSessionMetadata(sessionID string, _ map[string]any) (codexSessionMetadata, bool) {
	if strings.TrimSpace(sessionID) == "" {
		return codexSessionMetadata{}, false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return codexSessionMetadata{}, false
	}

	pattern := filepath.Join(homeDir, ".codex", "sessions", "*", "*", "*", "*"+sessionID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return codexSessionMetadata{}, false
	}
	sort.Strings(matches)

	for i := len(matches) - 1; i >= 0; i-- {
		metadata, ok := readCodexSessionMetadataFile(matches[i], sessionID)
		if ok {
			return metadata, true
		}
	}

	return codexSessionMetadata{}, false
}

func readCodexSessionMetadataFile(path string, sessionID string) (codexSessionMetadata, bool) {
	file, err := os.Open(path)
	if err != nil {
		return codexSessionMetadata{}, false
	}
	defer file.Close()

	line, err := bufio.NewReader(file).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return codexSessionMetadata{}, false
	}

	var entry struct {
		Type    string `json:"type"`
		Payload struct {
			ID           string `json:"id"`
			ForkedFromID string `json:"forked_from_id"`
			Source       struct {
				Subagent struct {
					ThreadSpawn struct {
						ParentThreadID string `json:"parent_thread_id"`
					} `json:"thread_spawn"`
				} `json:"subagent"`
			} `json:"source"`
			AgentNickname string `json:"agent_nickname"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &entry); err != nil {
		return codexSessionMetadata{}, false
	}
	if entry.Type != "session_meta" {
		return codexSessionMetadata{}, false
	}
	if strings.TrimSpace(entry.Payload.ID) != "" && entry.Payload.ID != sessionID {
		return codexSessionMetadata{}, false
	}

	parentSessionID := strings.TrimSpace(entry.Payload.Source.Subagent.ThreadSpawn.ParentThreadID)
	if parentSessionID == "" {
		parentSessionID = strings.TrimSpace(entry.Payload.ForkedFromID)
	}

	metadata := codexSessionMetadata{
		ParentSessionID: parentSessionID,
		IsSubagent:      parentSessionID != "",
		AgentNickname:   strings.TrimSpace(entry.Payload.AgentNickname),
	}
	return metadata, metadata.IsSubagent || metadata.AgentNickname != ""
}

func readCodexLastAssistantMessage(data map[string]any) (string, bool) {
	text := strings.TrimSpace(firstString(data, "last_assistant_message"))
	if text == "" {
		return "", false
	}
	return text, true
}

