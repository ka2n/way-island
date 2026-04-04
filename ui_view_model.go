package main

import (
	"encoding/base64"
	"strings"
)

const (
	panelViewClosed = 0
	panelViewList   = 1
	panelViewDetail = 2
)

type sessionRowViewModel struct {
	SessionID   string
	Title       string
	State       string
	StateClass  string
	StatusLabel string
	DetailText  string
}

type pillViewModel struct {
	Title      string
	State      string
	StateClass string
	Clickable  bool
	BadgeCount int
}

type detailViewModel struct {
	SessionID   string
	Title       string
	State       string
	StateClass  string
	StatusLabel string
	BodyText    string
}

type overlayViewModel struct {
	HasSessions bool
	Pill        pillViewModel
	ListTitle   string
	ListRows    []sessionRowViewModel
	Expanded    bool
	StackView   int
	Detail      *detailViewModel
}

type payloadSession struct {
	ID              string
	Name            string
	State           string
	Action          string
	LastUserMessage string
}

func buildOverlayViewModel(payload string, panelView int, selectedSessionID string) overlayViewModel {
	sessions := parsePayloadSessions(payload)
	hasSessions := len(sessions) > 0

	vm := overlayViewModel{
		HasSessions: hasSessions,
		Pill:        buildPillViewModel(sessions),
		ListTitle:   "Sessions",
		ListRows:    buildListRowsViewModel(sessions),
	}

	if !hasSessions {
		return vm
	}

	if panelView == panelViewClosed {
		return vm
	}

	vm.Expanded = true
	vm.StackView = panelViewList
	if panelView != panelViewDetail {
		return vm
	}

	detail := buildDetailViewModel(sessions, selectedSessionID)
	if detail == nil {
		detail = buildDetailViewModel(sessions, sessions[0].ID)
	}
	if detail == nil {
		return vm
	}

	vm.StackView = panelViewDetail
	vm.Detail = detail
	return vm
}

func buildPillViewModel(sessions []payloadSession) pillViewModel {
	if len(sessions) == 0 {
		return pillViewModel{
			Title:      "No sessions",
			State:      "idle",
			StateClass: statusClass("idle"),
		}
	}

	primary := sessions[0]
	return pillViewModel{
		Title:      primary.Name,
		State:      primary.State,
		StateClass: statusClass(primary.State),
		Clickable:  true,
		BadgeCount: len(sessions),
	}
}

func buildListRowsViewModel(sessions []payloadSession) []sessionRowViewModel {
	rows := make([]sessionRowViewModel, 0, len(sessions))
	for _, session := range sessions {
		rows = append(rows, sessionRowViewModel{
			SessionID:   session.ID,
			Title:       session.Name,
			State:       session.State,
			StateClass:  statusClass(session.State),
			StatusLabel: statusLabel(session.State),
			DetailText:  actionOrStatusLabel(session.Action, session.State),
		})
	}
	return rows
}

func buildDetailViewModel(sessions []payloadSession, selectedSessionID string) *detailViewModel {
	if selectedSessionID == "" {
		return nil
	}

	for _, session := range sessions {
		if session.ID != selectedSessionID {
			continue
		}
		return &detailViewModel{
			SessionID:   session.ID,
			Title:       session.Name,
			State:       session.State,
			StateClass:  statusClass(session.State),
			StatusLabel: statusLabel(session.State),
			BodyText:    detailBodyText(session.Action, session.LastUserMessage),
		}
	}

	return nil
}

func parsePayloadSessions(payload string) []payloadSession {
	if payload == "" {
		return nil
	}

	lines := strings.Split(payload, "\n")
	sessions := make([]payloadSession, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 3 {
			continue
		}

		sessionID, ok := decodePayloadField(fields[0])
		if !ok {
			continue
		}
		name, ok := decodePayloadField(fields[1])
		if !ok {
			continue
		}
		state, ok := decodePayloadField(fields[2])
		if !ok {
			continue
		}
		action := ""
		if len(fields) >= 4 {
			decodedAction, ok := decodePayloadField(fields[3])
			if ok {
				action = decodedAction
			}
		}
		lastUserMessage := ""
		if len(fields) >= 5 {
			decodedLastUserMessage, ok := decodePayloadField(fields[4])
			if ok {
				lastUserMessage = decodedLastUserMessage
			}
		}

		sessions = append(sessions, payloadSession{
			ID:              sessionID,
			Name:            name,
			State:           state,
			Action:          action,
			LastUserMessage: lastUserMessage,
		})
	}

	return sessions
}

func decodePayloadField(value string) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

func actionOrStatusLabel(action, state string) string {
	if action != "" {
		return action
	}
	return statusLabel(state)
}

func detailBodyText(action string, lastUserMessage string) string {
	switch {
	case action != "" && lastUserMessage != "":
		return action + "\n\nLast user message: " + lastUserMessage
	case action != "":
		return action
	case lastUserMessage != "":
		return "Last user message: " + lastUserMessage
	default:
		return ""
	}
}

func statusClass(state string) string {
	switch state {
	case "working":
		return "working"
	case "tool_running":
		return "tool-running"
	case "waiting":
		return "waiting"
	default:
		return "idle"
	}
}

func statusLabel(state string) string {
	switch state {
	case "working":
		return "Working"
	case "tool_running":
		return "Running tool"
	case "waiting":
		return "Waiting"
	default:
		return "Idle"
	}
}
