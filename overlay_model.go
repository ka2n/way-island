package main

import (
	"encoding/base64"
	"sort"
	"strings"

	"github.com/ka2n/way-island/internal/socket"
)

type overlayModel struct {
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

	switch update.Type {
	case socket.SessionUpdateUpsert:
		m.sessions[update.Session.ID] = update.Session
	case socket.SessionUpdateTimeout:
		delete(m.sessions, update.Session.ID)
	}
}

func (m *overlayModel) Payload() string {
	if len(m.sessions) == 0 {
		return ""
	}

	sessions := make([]socket.Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})

	var builder strings.Builder
	for _, session := range sessions {
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.ID)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.State)))
		builder.WriteByte('\n')
	}

	return builder.String()
}
