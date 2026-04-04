package main

import (
	"encoding/base64"
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
	if vm.Pill.BadgeCount != 2 {
		t.Fatalf("badge count = %d, want 2", vm.Pill.BadgeCount)
	}
	if got := vm.ListRows[0].DetailText; got != "Approval needed" {
		t.Fatalf("first row detail text = %q, want %q", got, "Approval needed")
	}
	if got := vm.ListRows[1].DetailText; got != "Working" {
		t.Fatalf("second row detail text = %q, want %q", got, "Working")
	}
}

func TestBuildOverlayViewModelDetailState(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{ID: "session-1", Name: "Alpha", State: "working", Action: "Running tests"},
		{ID: "session-2", Name: "Beta", State: "idle", Action: ""},
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
	if vm.Detail.BodyText != "" {
		t.Fatalf("detail body = %q", vm.Detail.BodyText)
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
		builder.WriteByte('\n')
	}
	return builder.String()
}
