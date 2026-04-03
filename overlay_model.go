package main

import (
	"encoding/base64"
	"sort"
	"strings"
	"sync"

	"github.com/ka2n/way-island/internal/socket"
)

// overlayModel is the single source of truth for session state (Flux-style store).
// Thread-safe: accessed from update goroutine, UI goroutine, and inspect handler.
type overlayModel struct {
	mu       sync.RWMutex
	sessions map[string]socket.Session
}

func newOverlayModel() *overlayModel {
	return &overlayModel{
		sessions: make(map[string]socket.Session),
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
		}
		m.sessions[update.Session.ID] = update.Session
	case socket.SessionUpdateTimeout:
		delete(m.sessions, update.Session.ID)
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
// Sessions are sorted by LastEventAt descending (most recent first).
// Format: base64(SessionID)\tbase64(DisplayName)\tbase64(State)\n per session.
func (m *overlayModel) Payload() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.sessions) == 0 {
		return ""
	}

	sessions := make([]socket.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].LastEventAt.Equal(sessions[j].LastEventAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].LastEventAt.After(sessions[j].LastEventAt)
	})

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
		builder.WriteByte('\n')
	}

	return builder.String()
}
