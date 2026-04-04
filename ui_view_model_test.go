package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildOverlayViewModelEmptyState(t *testing.T) {
	vm := buildOverlayViewModel("", panelViewClosed, "")

	if vm.HasSessions {
		t.Fatalf("expected no sessions")
	}
	if vm.Pill.Title != "No sessions" {
		t.Fatalf("pill title = %q, want %q", vm.Pill.Title, "No sessions")
	}
	if vm.Pill.StateClass != "idle" {
		t.Fatalf("pill state class = %q, want idle", vm.Pill.StateClass)
	}
	if vm.Expanded {
		t.Fatalf("expected collapsed empty state")
	}
	if len(vm.ListRows) != 0 {
		t.Fatalf("expected no list rows, got %d", len(vm.ListRows))
	}
}

func TestBuildOverlayViewModelListState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "waiting", Action: "Approval needed"},
		{ID: "session-1-child", Name: "Alpha", State: "working", Action: "Reviewing", ParentSessionID: "session-1", IsSubagent: true, AgentNickname: "Harvey"},
		{ID: "session-2", Name: "Beta", State: "working", Action: ""},
	})

	vm := buildOverlayViewModel(payload, panelViewList, "")

	if !vm.HasSessions {
		t.Fatalf("expected sessions")
	}
	if !vm.Expanded {
		t.Fatalf("expected expanded list state")
	}
	if vm.StackView != panelViewList {
		t.Fatalf("stack view = %d, want %d", vm.StackView, panelViewList)
	}
	if vm.Pill.Title != "Alpha" {
		t.Fatalf("pill title = %q, want Alpha", vm.Pill.Title)
	}
	if vm.Pill.BadgeCount != 3 {
		t.Fatalf("badge count = %d, want 3", vm.Pill.BadgeCount)
	}
	if vm.Pill.WaitingCount != 1 || vm.Pill.WorkingCount != 2 || vm.Pill.OtherCount != 0 {
		t.Fatalf("unexpected pill counts: waiting=%d working=%d other=%d", vm.Pill.WaitingCount, vm.Pill.WorkingCount, vm.Pill.OtherCount)
	}
	if got := vm.ListRows[0].DetailText; got != "Approval needed · SUBAGENTS 1" {
		t.Fatalf("first row detail text = %q, want %q", got, "Approval needed · SUBAGENTS 1")
	}
	if got := vm.ListRows[2].DetailText; got != "Working" {
		t.Fatalf("second row detail text = %q, want %q", got, "Working")
	}
}

func TestBuildOverlayViewModelPrefixesClaudeDisplayName(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "working", Action: "Running tests", HookSource: "claude"},
		{ID: "session-2", Name: "Beta", State: "idle", Action: "", HookSource: "codex"},
	})

	vm := buildOverlayViewModel(payload, panelViewDetail, "session-1")

	if vm.Pill.Title != "✳ Alpha" {
		t.Fatalf("pill title = %q, want %q", vm.Pill.Title, "✳ Alpha")
	}
	if vm.ListRows[0].Title != "✳ Alpha" {
		t.Fatalf("first row title = %q, want %q", vm.ListRows[0].Title, "✳ Alpha")
	}
	if vm.ListRows[1].Title != "Beta" {
		t.Fatalf("second row title = %q, want %q", vm.ListRows[1].Title, "Beta")
	}
	if vm.Detail == nil {
		t.Fatalf("expected detail view")
	}
	if vm.Detail.Title != "✳ Alpha" {
		t.Fatalf("detail title = %q, want %q", vm.Detail.Title, "✳ Alpha")
	}
}

func TestBuildOverlayViewModelDetailState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "working", Action: "Running tests"},
		{ID: "session-2", Name: "Beta", State: "idle", Action: "", AgentNickname: "Builder", HookSource: "codex"},
	})

	vm := buildOverlayViewModel(payload, panelViewDetail, "session-2")

	if !vm.Expanded {
		t.Fatalf("expected expanded detail state")
	}
	if vm.StackView != panelViewDetail {
		t.Fatalf("stack view = %d, want %d", vm.StackView, panelViewDetail)
	}
	if vm.Detail == nil {
		t.Fatalf("expected detail view")
	}
	if vm.Detail.Title != "Beta" {
		t.Fatalf("detail title = %q, want Beta", vm.Detail.Title)
	}
	if vm.Detail.Agent != "Codex" {
		t.Fatalf("detail agent = %q, want Codex", vm.Detail.Agent)
	}
	if vm.Detail.AgentName != "Builder" {
		t.Fatalf("detail agent name = %q, want Builder", vm.Detail.AgentName)
	}
	if vm.Detail.BodyText != "" {
		t.Fatalf("detail body = %q", vm.Detail.BodyText)
	}
}

func TestBuildOverlayViewModelDetailStateForClaude(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "working", Action: "Running tests", HookSource: "claude", AgentNickname: "Ignored"},
	})

	vm := buildOverlayViewModel(payload, panelViewDetail, "session-1")

	if vm.Detail == nil {
		t.Fatalf("expected detail view")
	}
	if vm.Detail.Agent != "Claude Code" {
		t.Fatalf("detail agent = %q, want Claude Code", vm.Detail.Agent)
	}
	if vm.Detail.AgentName != "" {
		t.Fatalf("detail agent name = %q, want empty", vm.Detail.AgentName)
	}
}

func TestBuildOverlayViewModelDetailFallsBackToPrimarySession(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "tool_running", Action: "Bash: go test ./..."},
		{ID: "session-2", Name: "Beta", State: "idle", Action: ""},
	})

	vm := buildOverlayViewModel(payload, panelViewDetail, "missing-session")

	if vm.Detail == nil {
		t.Fatalf("expected detail view")
	}
	if vm.Detail.SessionID != "session-1" {
		t.Fatalf("detail session = %q, want session-1", vm.Detail.SessionID)
	}
	if vm.Detail.StatusLabel != "Running tool" {
		t.Fatalf("detail status label = %q, want %q", vm.Detail.StatusLabel, "Running tool")
	}
	if vm.Detail.BodyText != "Bash: go test ./..." {
		t.Fatalf("detail body = %q, want action text", vm.Detail.BodyText)
	}
}

func TestBuildOverlayViewModelDetailIncludesSubagents(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "working", Action: "Running tests", Subagents: []payloadSubagent{
			{ID: "agent-a9a5daa5eb7af9fd8", Title: "agent-a9a5daa5eb7af9fd8", Description: "Claude latest version research", State: "tool_running"},
			{ID: "agent-a44db7587a2a21be3", Title: "agent-a44db7587a2a21be3", Description: "Claude latest version research", State: "idle"},
		}},
	})

	vm := buildOverlayViewModel(payload, panelViewDetail, "session-1")
	if vm.Detail == nil {
		t.Fatalf("expected detail view")
	}
	if vm.Detail.SubagentCount != 2 {
		t.Fatalf("subagent count = %d, want 2", vm.Detail.SubagentCount)
	}
	if len(vm.Detail.SubagentRows) != 2 {
		t.Fatalf("subagent rows = %d, want 2", len(vm.Detail.SubagentRows))
	}
	if vm.Detail.SubagentRows[0].Title != "agent-a9a5daa5eb7af9fd8" {
		t.Fatalf("first subagent title = %q", vm.Detail.SubagentRows[0].Title)
	}
	if vm.Detail.SubagentRows[0].DetailText != "Claude latest version research · Running tool" {
		t.Fatalf("first subagent detail = %q", vm.Detail.SubagentRows[0].DetailText)
	}
}

func TestParsePayloadSessionsSkipsInvalidRows(t *testing.T) {
	valid := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "idle", Action: ""},
	})
	payload := strings.Join([]string{
		"not-base64",
		valid,
	}, "\n")

	sessions := parsePayloadSessions(payload)
	if len(sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(sessions))
	}
	if sessions[0].ID != "session-1" {
		t.Fatalf("session id = %q, want session-1", sessions[0].ID)
	}
}

func encodePayloadSessions(sessions []payloadSession) string {
	var builder strings.Builder
	for _, session := range sessions {
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.ID)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.Name)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.State)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.Action)))
		builder.WriteByte('\t')
		builder.WriteString(base64.StdEncoding.EncodeToString([]byte(session.LastUserMessage)))
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
		subagentsJSON, err := json.Marshal(session.Subagents)
		if err != nil {
			panic(err)
		}
		builder.WriteString(base64.StdEncoding.EncodeToString(subagentsJSON))
		builder.WriteByte('\n')
	}
	return builder.String()
}
