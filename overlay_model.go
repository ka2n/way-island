package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/ka2n/way-island/internal/socket"
)

// overlayModel is the single source of truth for session state (Flux-style store).
// Thread-safe: accessed from update goroutine, UI goroutine, and inspect handler.
type overlayModel struct {
	mu            sync.RWMutex
	sessions      map[string]socket.Session
	order         []string        // stable insertion order; new sessions appended to end
	suppressedIDs map[string]bool // sessions whose pane is currently the active tmux pane
}

const maxLastUserMessageRunes = 120

func newOverlayModel() *overlayModel {
	return &overlayModel{
		sessions:      make(map[string]socket.Session),
		suppressedIDs: make(map[string]bool),
	}
}

// SetSuppressed marks or unmarks a session as suppressed.
// Suppressed sessions do not contribute to the active-session badge counts,
// because the user's terminal is already showing that session.
func (m *overlayModel) SetSuppressed(id string, suppressed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if suppressed {
		m.suppressedIDs[id] = true
	} else {
		delete(m.suppressedIDs, id)
	}
}

func (m *overlayModel) Apply(update socket.SessionUpdate) {
	if update.Session.ID == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch update.Type {
	case socket.SessionUpdateUpsert:
		// Don't overwrite with older data (hooks are authoritative over JSONL)
		if existing, ok := m.sessions[update.Session.ID]; ok {
			if update.Session.LastEventAt.Before(existing.LastEventAt) {
				return
			}
		} else {
			// New session: append to stable order
			m.order = append(m.order, update.Session.ID)
		}
		m.sessions[update.Session.ID] = update.Session
	case socket.SessionUpdateTimeout:
		delete(m.sessions, update.Session.ID)
		delete(m.suppressedIDs, update.Session.ID)
		// Remove from stable order
		for i, id := range m.order {
			if id == update.Session.ID {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
	}
}

// Sessions returns a snapshot of all sessions (implements socket.Inspector).
func (m *overlayModel) Sessions() map[string]socket.Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make(map[string]socket.Session, len(m.sessions))
	for id, session := range m.sessions {
		snapshot[id] = session
	}
	return snapshot
}

func (m *overlayModel) Session(id string) (socket.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[id]
	return session, ok
}

// Payload returns a base64-encoded TSV string for the GTK UI.
// Sessions are returned in stable insertion order; new sessions are appended to the end.
// Format (12 tab-separated base64 fields per line):
//
//	base64(SessionID)\tbase64(DisplayName)\tbase64(State)\tbase64(CurrentAction)\t
//	base64(LastUserMessage)\tbase64(ParentSessionID)\tbase64(IsSubagent "0"|"1")\t
//	base64(AgentNickname)\tbase64(HookSource)\tbase64(Subagents JSON)\t
//	base64(IsSuppressed "0"|"1")\tbase64(LastAssistantMessage)\n
func (m *overlayModel) Payload() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.sessions) == 0 {
		return ""
	}

	// Use stable insertion order — no reordering while list is open.
	// New sessions are appended to the end.
	sessions := make([]socket.Session, 0, len(m.sessions))
	for _, id := range m.order {
		if session, ok := m.sessions[id]; ok {
			sessions = append(sessions, session)
		}
	}

	var builder strings.Builder
	for _, session := range sessions {
		name := session.DisplayName
		if name == "" {
			name = session.ID[:8]
		}
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.ID)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(name)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.State)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.CurrentAction)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(truncateLastUserMessage(session.LastUserMessage))))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.ParentSessionID)))
		builder.WriteByte('\t')
		if session.IsSubagent {
			builder.WriteString(base64.StdEncoding.EncodeToString([]byte("1")))
		} else {
			builder.WriteString(base64.StdEncoding.EncodeToString([]byte("0")))
		}
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.AgentNickname)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.HookSource)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString(mustMarshalSubagents(session.Subagents)))
		builder.WriteByte('\t')
		if m.suppressedIDs[session.ID] {
			builder.WriteString(base64.StdEncoding.EncodeToString([]byte("1")))
		} else {
			builder.WriteString(base64.StdEncoding.EncodeToString([]byte("0")))
		}
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(truncateLastUserMessage(session.LastAssistantMessage))))
		builder.WriteByte('\n')
	}

	return builder.String()
}

func mustMarshalSubagents(subagents []socket.SubagentSummary) []byte {
	if len(subagents) == 0 {
		return []byte("[]")
	}
	data, err := json.Marshal(subagents)
	if err != nil {
		return []byte("[]")
	}
	return data
}

func truncateLastUserMessage(message string) string {
	if utf8.RuneCountInString(message) <= maxLastUserMessageRunes {
		return message
	}

	runes := []rune(message)
	return string(runes[:maxLastUserMessageRunes]) + "..."
}
