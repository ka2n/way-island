package socket

import (
	"context"
	"sync"
	"time"
)

const DefaultSessionTimeout = 30 * time.Second

type SessionState string

const (
	SessionStateIdle        SessionState = "idle"
	SessionStateWorking     SessionState = "working"
	SessionStateToolRunning SessionState = "tool_running"
	SessionStateWaiting     SessionState = "waiting"
)

type Session struct {
	ID          string
	State       SessionState
	LastEventAt time.Time
}

type SessionUpdateType string

const (
	SessionUpdateUpsert  SessionUpdateType = "upsert"
	SessionUpdateTimeout SessionUpdateType = "timeout"
)

type SessionUpdate struct {
	Type    SessionUpdateType
	Session Session
}

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]Session
	timeout  time.Duration
	now      func() time.Time
	updates  chan SessionUpdate
}

func NewSessionManager(timeout time.Duration) *SessionManager {
	if timeout <= 0 {
		timeout = DefaultSessionTimeout
	}

	return &SessionManager{
		sessions: make(map[string]Session),
		timeout:  timeout,
		now:      time.Now,
		updates:  make(chan SessionUpdate, 32),
	}
}

func (m *SessionManager) Updates() <-chan SessionUpdate {
	return m.updates
}

func (m *SessionManager) HandleMessage(message Message) {
	if message.Event == "session_end" {
		m.removeSession(message.SessionID)
		return
	}

	state, ok := sessionStateFromEvent(message.Event)
	if !ok {
		return
	}

	session := Session{
		ID:          message.SessionID,
		State:       state,
		LastEventAt: m.now(),
	}

	m.mu.Lock()
	m.sessions[message.SessionID] = session
	m.mu.Unlock()

	m.notify(SessionUpdate{
		Type:    SessionUpdateUpsert,
		Session: session,
	})
}

func (m *SessionManager) removeSession(sessionID string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if !ok {
		return
	}

	m.notify(SessionUpdate{
		Type:    SessionUpdateTimeout,
		Session: session,
	})
}

func (m *SessionManager) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = m.timeout
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				m.pruneExpired(now)
			}
		}
	}()
}

func (m *SessionManager) Sessions() map[string]Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := make(map[string]Session, len(m.sessions))
	for id, session := range m.sessions {
		sessions[id] = session
	}

	return sessions
}

func (m *SessionManager) pruneExpired(now time.Time) {
	var expired []Session

	m.mu.Lock()
	for id, session := range m.sessions {
		if now.Sub(session.LastEventAt) < m.timeout {
			continue
		}

		expired = append(expired, session)
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	for _, session := range expired {
		m.notify(SessionUpdate{
			Type:    SessionUpdateTimeout,
			Session: session,
		})
	}
}

func (m *SessionManager) notify(update SessionUpdate) {
	select {
	case m.updates <- update:
	default:
	}
}

func sessionStateFromEvent(event string) (SessionState, bool) {
	switch event {
	case string(SessionStateIdle):
		return SessionStateIdle, true
	case string(SessionStateWorking):
		return SessionStateWorking, true
	case string(SessionStateToolRunning):
		return SessionStateToolRunning, true
	case string(SessionStateWaiting):
		return SessionStateWaiting, true
	case "session_start":
		return SessionStateWorking, true
	case "tool_start":
		return SessionStateToolRunning, true
	case "tool_end":
		return SessionStateWorking, true
	case "response":
		return SessionStateWaiting, true
	default:
		return "", false
	}
}
