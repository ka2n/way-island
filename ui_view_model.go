package main

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
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
	Title        string
	State        string
	StateClass   string
	Clickable    bool
	BadgeCount   int
	WaitingCount int
	WorkingCount int
	OtherCount   int
}

type detailViewModel struct {
	SessionID     string
	Agent         string
	AgentName     string
	Title         string
	State         string
	StateClass    string
	StatusLabel   string
	BodyText      string
	SubagentCount int
	SubagentRows  []sessionRowViewModel
}

type overlayViewModel struct {
	HasSessions    bool
	Pill           pillViewModel
	ListTitle      string
	ListRows       []sessionRowViewModel
	Expanded       bool
	StackView      int
	BackdropActive bool
	Detail         *detailViewModel
}

type payloadSession struct {
	ID              string
	Name            string
	State           string
	Action          string
	LastUserMessage string
	ParentSessionID string
	IsSubagent      bool
	AgentNickname   string
	HookSource      string
	Subagents       []payloadSubagent
	IsSuppressed    bool
}

type payloadSubagent struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
}

func buildOverlayViewModel(payload string, panelView int, selectedSessionID string, panelPinned bool) overlayViewModel {
	sessions := parsePayloadSessions(payload)
	hasSessions := len(sessions) > 0

	vm := overlayViewModel{
		HasSessions:    hasSessions,
		Pill:           buildPillViewModel(sessions),
		ListTitle:      "Sessions",
		ListRows:       buildListRowsViewModel(sessions),
		BackdropActive: panelPinned && hasSessions && panelView != panelViewClosed,
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
		Title:      displayName(primary),
		State:      primary.State,
		StateClass: statusClass(primary.State),
		Clickable:  true,
		BadgeCount: len(sessions),
		// Suppressed sessions are already visible in the user's terminal,
		// so they should not contribute to the "needs attention" badge counts.
		WaitingCount: countSessionsByClass(sessions, "waiting"),
		WorkingCount: countSessionsByClass(sessions, "working"),
		OtherCount:   countSessionsByClass(sessions, "other"),
	}
}

func buildListRowsViewModel(sessions []payloadSession) []sessionRowViewModel {
	rows := make([]sessionRowViewModel, 0, len(sessions))
	for _, session := range sessions {
		detailText := actionOrStatusLabel(session.Action, session.State)
		if subagentCount := countDirectSubagents(sessions, session.ID); subagentCount > 0 {
			detailText += " · SUBAGENTS " + strconv.Itoa(subagentCount)
		}
		rows = append(rows, sessionRowViewModel{
			SessionID:   session.ID,
			Title:       displayName(session),
			State:       session.State,
			StateClass:  statusClass(session.State),
			StatusLabel: statusLabel(session.State),
			DetailText:  detailText,
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
			SessionID:     session.ID,
			Agent:         detailAgent(session),
			AgentName:     detailAgentName(session),
			Title:         displayName(session),
			State:         session.State,
			StateClass:    statusClass(session.State),
			StatusLabel:   statusLabel(session.State),
			BodyText:      detailBodyText(session.Action, session.LastUserMessage),
			SubagentCount: countDirectSubagents(sessions, session.ID),
			SubagentRows:  buildSubagentRowsViewModel(sessions, session.ID),
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

		fields := strings.SplitN(line, "\t", 11)
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
		parentSessionID := ""
		if len(fields) >= 6 {
			decodedParentSessionID, ok := decodePayloadField(fields[5])
			if ok {
				parentSessionID = decodedParentSessionID
			}
		}
		isSubagent := false
		if len(fields) >= 7 {
			decodedIsSubagent, ok := decodePayloadField(fields[6])
			if ok {
				isSubagent = decodedIsSubagent == "1"
			}
		}
		agentNickname := ""
		if len(fields) >= 8 {
			decodedAgentNickname, ok := decodePayloadField(fields[7])
			if ok {
				agentNickname = decodedAgentNickname
			}
		}
		hookSource := ""
		if len(fields) >= 9 {
			decodedHookSource, ok := decodePayloadField(fields[8])
			if ok {
				hookSource = decodedHookSource
			}
		}
		subagents := []payloadSubagent(nil)
		if len(fields) >= 10 {
			decodedSubagents, ok := decodePayloadField(fields[9])
			if ok && decodedSubagents != "" {
				_ = json.Unmarshal([]byte(decodedSubagents), &subagents)
			}
		}
		isSuppressed := false
		if len(fields) >= 11 {
			decodedSuppressed, ok := decodePayloadField(fields[10])
			if ok {
				isSuppressed = decodedSuppressed == "1"
			}
		}

		sessions = append(sessions, payloadSession{
			ID:              sessionID,
			Name:            name,
			State:           state,
			Action:          action,
			LastUserMessage: lastUserMessage,
			ParentSessionID: parentSessionID,
			IsSubagent:      isSubagent,
			AgentNickname:   agentNickname,
			HookSource:      hookSource,
			Subagents:       subagents,
			IsSuppressed:    isSuppressed,
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

func buildSubagentRowsViewModel(sessions []payloadSession, parentSessionID string) []sessionRowViewModel {
	if session, ok := findPayloadSessionByID(sessions, parentSessionID); ok && len(session.Subagents) > 0 {
		rows := make([]sessionRowViewModel, 0, len(session.Subagents))
		for _, subagent := range session.Subagents {
			state := subagent.State
			if strings.TrimSpace(state) == "" {
				state = "idle"
			}
			detailText := strings.TrimSpace(subagent.Description)
			statusText := statusLabel(state)
			if detailText == "" {
				detailText = statusLabel(state)
			} else if state != "idle" {
				detailText += " · " + statusText
			}
			rows = append(rows, sessionRowViewModel{
				SessionID:   subagent.ID,
				Title:       subagent.Title,
				State:       state,
				StateClass:  statusClass(state),
				StatusLabel: statusLabel(state),
				DetailText:  detailText,
			})
		}
		return rows
	}

	rows := make([]sessionRowViewModel, 0)
	for _, session := range sessions {
		if session.ParentSessionID != parentSessionID {
			continue
		}
		rows = append(rows, sessionRowViewModel{
			SessionID:   session.ID,
			Title:       subagentTitle(session),
			State:       session.State,
			StateClass:  statusClass(session.State),
			StatusLabel: statusLabel(session.State),
			DetailText:  actionOrStatusLabel(session.Action, session.State),
		})
	}
	return rows
}

func countDirectSubagents(sessions []payloadSession, parentSessionID string) int {
	if session, ok := findPayloadSessionByID(sessions, parentSessionID); ok && len(session.Subagents) > 0 {
		return len(session.Subagents)
	}

	count := 0
	for _, session := range sessions {
		if session.ParentSessionID == parentSessionID {
			count++
		}
	}
	return count
}

func findPayloadSessionByID(sessions []payloadSession, sessionID string) (payloadSession, bool) {
	for _, session := range sessions {
		if session.ID == sessionID {
			return session, true
		}
	}
	return payloadSession{}, false
}

func subagentTitle(session payloadSession) string {
	switch {
	case session.AgentNickname != "":
		return session.AgentNickname
	case session.Name != "":
		return session.Name
	default:
		return session.ID
	}
}

func detailAgent(session payloadSession) string {
	switch strings.TrimSpace(session.HookSource) {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude Code"
	default:
		return ""
	}
}

func detailAgentName(session payloadSession) string {
	if strings.TrimSpace(session.HookSource) != "codex" {
		return ""
	}
	return strings.TrimSpace(session.AgentNickname)
}

func displayName(session payloadSession) string {
	name := strings.TrimSpace(session.Name)
	if detailAgent(session) == "Claude Code" && name != "" {
		return "✳ " + name
	}
	return name
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

func countSessionsByClass(sessions []payloadSession, group string) int {
	count := 0
	for _, session := range sessions {
		// Suppressed sessions are already visible in the user's active terminal pane.
		// Don't count them as needing attention in the badge.
		if session.IsSuppressed {
			continue
		}
		sessionGroup := "other"
		switch session.State {
		case "waiting":
			sessionGroup = "waiting"
		case "working":
			sessionGroup = "working"
		}
		if sessionGroup == group {
			count++
		}
	}
	return count
}

func encodeViewField(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

func serializePillViewModel(vm overlayViewModel) string {
	fields := []string{
		encodeViewField(vm.Pill.Title),
		encodeViewField(vm.Pill.StateClass),
		strconv.Itoa(boolToInt(vm.Pill.Clickable)),
		strconv.Itoa(vm.Pill.BadgeCount),
		strconv.Itoa(vm.Pill.WaitingCount),
		strconv.Itoa(vm.Pill.WorkingCount),
		strconv.Itoa(vm.Pill.OtherCount),
	}
	return strings.Join(fields, "\t")
}

func serializeListViewModel(vm overlayViewModel) string {
	var lines []string
	lines = append(lines, encodeViewField(vm.ListTitle))
	for _, row := range vm.ListRows {
		lines = append(lines, strings.Join([]string{
			encodeViewField(row.SessionID),
			encodeViewField(row.Title),
			encodeViewField(row.StateClass),
			encodeViewField(row.DetailText),
		}, "\t"))
	}
	return strings.Join(lines, "\n")
}

func serializeDetailViewModel(vm overlayViewModel) string {
	if vm.Detail == nil {
		return ""
	}

	lines := []string{strings.Join([]string{
		encodeViewField(vm.Detail.SessionID),
		encodeViewField(vm.Detail.Title),
		encodeViewField(vm.Detail.StateClass),
		encodeViewField(vm.Detail.StatusLabel),
		encodeViewField(vm.Detail.BodyText),
		strconv.Itoa(vm.Detail.SubagentCount),
	}, "\t")}
	for _, row := range vm.Detail.SubagentRows {
		lines = append(lines, strings.Join([]string{
			encodeViewField(row.SessionID),
			encodeViewField(row.Title),
			encodeViewField(row.StateClass),
			encodeViewField(row.DetailText),
		}, "\t"))
	}
	return strings.Join(lines, "\n")
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
