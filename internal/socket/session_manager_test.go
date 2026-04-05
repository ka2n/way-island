package socket

import (
	"strings"
	"testing"
	"time"
)

type fakeSessionProcessMonitor struct {
	closeCount int
	onClose    func()
}

func (m *fakeSessionProcessMonitor) Close() error {
	m.closeCount++
	if m.onClose != nil {
		m.onClose()
	}
	return nil
}

func TestSessionManagerAddsSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager(DefaultSessionTimeout)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateUpsert {
		t.Fatalf("unexpected update type: %q", update.Type)
	}
	if update.Session.ID != "session-1" {
		t.Fatalf("unexpected session ID: %q", update.Session.ID)
	}
	if update.Session.State != SessionStateWorking {
		t.Fatalf("unexpected session state: %q", update.Session.State)
	}

	sessions := manager.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if session := sessions["session-1"]; session.LastEventAt != now {
		t.Fatalf("unexpected last event time: %v", session.LastEventAt)
	}
}

func TestSessionManagerUpdatesExistingSession(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	now := base
	manager := NewSessionManager(DefaultSessionTimeout)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	now = base.Add(5 * time.Second)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "tool_running",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateToolRunning {
		t.Fatalf("unexpected session state: %q", update.Session.State)
	}
	if update.Session.LastEventAt != now {
		t.Fatalf("unexpected update timestamp: %v", update.Session.LastEventAt)
	}

	sessions := manager.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if session := sessions["session-1"]; session.State != SessionStateToolRunning {
		t.Fatalf("unexpected stored state: %q", session.State)
	}
}

func TestSessionManagerStoresCurrentActionForToolStart(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "tool_start",
		Data: map[string]any{
			"tool_name":    "Bash",
			"command":      "go test ./...",
			"_hook_source": "codex",
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.CurrentAction != "Bash: go test ./..." {
		t.Fatalf("CurrentAction = %q, want %q", update.Session.CurrentAction, "Bash: go test ./...")
	}
	if update.Session.CurrentTool != "bash" {
		t.Fatalf("CurrentTool = %q, want %q", update.Session.CurrentTool, "bash")
	}
	if update.Session.HookSource != "codex" {
		t.Fatalf("HookSource = %q, want %q", update.Session.HookSource, "codex")
	}
}

func TestSessionManagerStoresLastUserMessageFromPromptSubmit(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"hook_event_name": "UserPromptSubmit",
			"prompt":          "  show me the last user message in session detail  ",
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.LastUserMessage != "show me the last user message in session detail" {
		t.Fatalf("LastUserMessage = %q", update.Session.LastUserMessage)
	}
}

func TestSessionManagerTruncatesLastUserMessageByRune(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"hook_event_name": "UserPromptSubmit",
			"prompt":          strings.Repeat("あ", maxLastUserMessageRunes),
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.LastUserMessage != strings.Repeat("あ", maxLastUserMessageRunes) {
		t.Fatalf("LastUserMessage = %q", update.Session.LastUserMessage)
	}

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"hook_event_name": "UserPromptSubmit",
			"prompt":          strings.Repeat("あ", maxLastUserMessageRunes+1),
		},
	})

	update = waitForSessionUpdate(t, manager.Updates())
	want := strings.Repeat("あ", maxLastUserMessageRunes-3) + "..."
	if update.Session.LastUserMessage != want {
		t.Fatalf("LastUserMessage = %q, want %q", update.Session.LastUserMessage, want)
	}
}

func TestSessionManagerKeepsLastUserMessageWhenWorkingUpdateIsNotPromptSubmit(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"hook_event_name": "UserPromptSubmit",
			"prompt":          "initial prompt",
		},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.LastUserMessage != "initial prompt" {
		t.Fatalf("LastUserMessage = %q, want %q", update.Session.LastUserMessage, "initial prompt")
	}
}

func TestSessionManagerClearsCurrentActionAfterToolEnd(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "tool_start",
		Data: map[string]any{
			"tool_name": "Bash",
			"command":   "go test ./...",
		},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "tool_end",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.CurrentAction != "" {
		t.Fatalf("CurrentAction = %q, want empty", update.Session.CurrentAction)
	}
	if update.Session.CurrentTool != "" {
		t.Fatalf("CurrentTool = %q, want empty", update.Session.CurrentTool)
	}
}

func TestSessionManagerStoresAgentMetadata(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"_ppid":               float64(42),
			"_agent_pid_ns_inode": float64(4026533000),
			"_agent_start_time":   float64(123456),
			"_agent_tty":          "/dev/pts/9",
			"_agent_tty_nr":       float64(34825),
			"_hook_tty":           "/dev/pts/11",
			"_jai_jail":           true,
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.AgentPID != 42 {
		t.Fatalf("AgentPID = %d, want 42", update.Session.AgentPID)
	}
	if update.Session.AgentPIDNamespaceInode != 4026533000 {
		t.Fatalf("AgentPIDNamespaceInode = %d, want 4026533000", update.Session.AgentPIDNamespaceInode)
	}
	if update.Session.AgentStartTimeTicks != 123456 {
		t.Fatalf("AgentStartTimeTicks = %d, want 123456", update.Session.AgentStartTimeTicks)
	}
	if update.Session.AgentTTY != "/dev/pts/9" {
		t.Fatalf("AgentTTY = %q, want /dev/pts/9", update.Session.AgentTTY)
	}
	if update.Session.AgentTTYNr != 34825 {
		t.Fatalf("AgentTTYNr = %d, want 34825", update.Session.AgentTTYNr)
	}
	if update.Session.HookTTY != "/dev/pts/11" {
		t.Fatalf("HookTTY = %q, want /dev/pts/11", update.Session.HookTTY)
	}
	if !update.Session.AgentInJail {
		t.Fatalf("AgentInJail = false, want true")
	}
}

func TestSessionManagerRemovesSessionOnProcessExit(t *testing.T) {
	origFactory := newSessionProcessMonitor
	t.Cleanup(func() {
		newSessionProcessMonitor = origFactory
	})

	exitCallbacks := map[string]func(){}
	newSessionProcessMonitor = func(session Session, onExit func()) (sessionProcessMonitor, error) {
		exitCallbacks[session.ID] = onExit
		return &fakeSessionProcessMonitor{}, nil
	}

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"_ppid":             float64(42),
			"_agent_start_time": float64(123456),
		},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	onExit, ok := exitCallbacks["session-1"]
	if !ok {
		t.Fatal("expected process exit callback to be registered")
	}
	onExit()

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateTimeout {
		t.Fatalf("unexpected update type: %q", update.Type)
	}
	if update.Reason != "pidfd:process_exit" {
		t.Fatalf("unexpected removal reason: %q", update.Reason)
	}
	if len(manager.Sessions()) != 0 {
		t.Fatalf("expected session to be removed, got %d sessions", len(manager.Sessions()))
	}
}

func TestSessionManagerIgnoresStaleProcessExitAfterPIDChange(t *testing.T) {
	origFactory := newSessionProcessMonitor
	t.Cleanup(func() {
		newSessionProcessMonitor = origFactory
	})

	monitors := make(map[int]*fakeSessionProcessMonitor)
	exitCallbacks := make(map[int]func())
	newSessionProcessMonitor = func(session Session, onExit func()) (sessionProcessMonitor, error) {
		monitor := &fakeSessionProcessMonitor{}
		monitors[session.AgentPID] = monitor
		exitCallbacks[session.AgentPID] = onExit
		return monitor, nil
	}

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"_ppid":             float64(42),
			"_agent_start_time": float64(1000),
		},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"_ppid":             float64(84),
			"_agent_start_time": float64(2000),
		},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	if monitors[42].closeCount != 1 {
		t.Fatalf("old monitor close count = %d, want 1", monitors[42].closeCount)
	}

	exitCallbacks[42]()

	sessions := manager.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected updated session to remain, got %d sessions", len(sessions))
	}
	if sessions["session-1"].AgentPID != 84 {
		t.Fatalf("AgentPID = %d, want 84", sessions["session-1"].AgentPID)
	}
}

func TestSessionManagerEnrichesCodexSubagentMetadata(t *testing.T) {
	orig := readCodexSessionMetadataFunc
	readCodexSessionMetadataFunc = func(sessionID string) (codexSessionMetadata, bool) {
		if sessionID != "session-1" {
			return codexSessionMetadata{}, false
		}
		return codexSessionMetadata{
			ParentSessionID: "parent-1",
			IsSubagent:      true,
			AgentNickname:   "Harvey",
		}, true
	}
	t.Cleanup(func() {
		readCodexSessionMetadataFunc = orig
	})

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"_hook_source": "codex",
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if !update.Session.IsSubagent {
		t.Fatalf("IsSubagent = false, want true")
	}
	if update.Session.ParentSessionID != "parent-1" {
		t.Fatalf("ParentSessionID = %q, want %q", update.Session.ParentSessionID, "parent-1")
	}
	if update.Session.AgentNickname != "Harvey" {
		t.Fatalf("AgentNickname = %q, want %q", update.Session.AgentNickname, "Harvey")
	}
}

func TestSessionManagerEnrichesClaudeSubagentMetadata(t *testing.T) {
	orig := readClaudeSessionMetadataFunc
	callCount := 0
	readClaudeSessionMetadataFunc = func(sessionID string, cwd string) (claudeSessionMetadata, bool) {
		callCount++
		if sessionID != "session-1" || cwd != "/tmp/project" {
			return claudeSessionMetadata{}, false
		}
		state := string(SessionStateIdle)
		if callCount == 1 {
			state = string(SessionStateToolRunning)
		}
		return claudeSessionMetadata{
			Subagents: []SubagentSummary{
				{ID: "agent-a9a5daa5eb7af9fd8", Title: "agent-a9a5daa5eb7af9fd8", Description: "Claude latest version research", State: state},
			},
		}, true
	}
	t.Cleanup(func() {
		readClaudeSessionMetadataFunc = orig
	})

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data: map[string]any{
			"_hook_source": "claude",
			"cwd":          "/tmp/project",
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if len(update.Session.Subagents) != 1 {
		t.Fatalf("Subagents len = %d, want 1", len(update.Session.Subagents))
	}
	if update.Session.Subagents[0].ID != "agent-a9a5daa5eb7af9fd8" {
		t.Fatalf("Subagents[0].ID = %q", update.Session.Subagents[0].ID)
	}
	if update.Session.Subagents[0].State != string(SessionStateToolRunning) {
		t.Fatalf("Subagents[0].State = %q", update.Session.Subagents[0].State)
	}

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "tool_end",
		Data: map[string]any{
			"_hook_source": "claude",
			"cwd":          "/tmp/project",
		},
	})

	update = waitForSessionUpdate(t, manager.Updates())
	if update.Session.Subagents[0].State != string(SessionStateIdle) {
		t.Fatalf("Subagents[0].State after refresh = %q", update.Session.Subagents[0].State)
	}
}

func TestSessionManagerTimesOutInactiveSessions(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	now := base
	manager := NewSessionManager(30 * time.Second)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "waiting",
		Data:      map[string]any{},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	now = base.Add(31 * time.Second)
	manager.pruneExpired(now)

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateTimeout {
		t.Fatalf("unexpected update type: %q", update.Type)
	}
	if update.Session.ID != "session-1" {
		t.Fatalf("unexpected timed-out session: %q", update.Session.ID)
	}

	sessions := manager.Sessions()
	if len(sessions) != 0 {
		t.Fatalf("expected timed-out session to be removed, got %d sessions", len(sessions))
	}
}

func TestSessionManagerMapsClaudeHookEvents(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "session_start",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateWorking {
		t.Fatalf("unexpected session state: %q", update.Session.State)
	}

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "response",
		Data:      map[string]any{},
	})

	update = waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateWaiting {
		t.Fatalf("unexpected response state: %q", update.Session.State)
	}
}

func TestSessionManagerTreatsRootIdleHookAsWaiting(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "idle",
		Data: map[string]any{
			"_hook_source": "codex",
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateWaiting {
		t.Fatalf("Session.State = %q, want %q", update.Session.State, SessionStateWaiting)
	}
}

func TestSessionManagerKeepsSubagentIdleHookIdle(t *testing.T) {
	t.Parallel()

	orig := readCodexSessionMetadataFunc
	readCodexSessionMetadataFunc = func(sessionID string) (codexSessionMetadata, bool) {
		if sessionID != "session-1" {
			return codexSessionMetadata{}, false
		}
		return codexSessionMetadata{
			ParentSessionID: "parent-1",
			IsSubagent:      true,
		}, true
	}
	t.Cleanup(func() {
		readCodexSessionMetadataFunc = orig
	})

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "idle",
		Data: map[string]any{
			"_hook_source": "codex",
		},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Session.State != SessionStateIdle {
		t.Fatalf("Session.State = %q, want %q", update.Session.State, SessionStateIdle)
	}
}

func TestSessionManagerRemovesSessionOnSessionEnd(t *testing.T) {
	t.Parallel()

	manager := NewSessionManager(DefaultSessionTimeout)
	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "working",
		Data:      map[string]any{},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "session_end",
		Data:      map[string]any{},
	})

	update := waitForSessionUpdate(t, manager.Updates())
	if update.Type != SessionUpdateTimeout {
		t.Fatalf("unexpected update type: %q", update.Type)
	}

	sessions := manager.Sessions()
	if len(sessions) != 0 {
		t.Fatalf("expected session to be removed, got %d sessions", len(sessions))
	}
}

func TestSessionManagerKeepsNamespacedSessionAlive(t *testing.T) {
	t.Parallel()

	origList := listProcPIDsForLiveness
	origReadNS := readPIDNamespaceInodeForLiveness
	origReadNSpid := readNSPIDsForLiveness
	origReadStat := readProcStatForLiveness
	origAlive := isProcessAliveFunc
	t.Cleanup(func() {
		listProcPIDsForLiveness = origList
		readPIDNamespaceInodeForLiveness = origReadNS
		readNSPIDsForLiveness = origReadNSpid
		readProcStatForLiveness = origReadStat
		isProcessAliveFunc = origAlive
	})

	listProcPIDsForLiveness = func() ([]int, error) { return []int{3210}, nil }
	readPIDNamespaceInodeForLiveness = func(pid int) (uint64, error) { return 4026533000, nil }
	readNSPIDsForLiveness = func(pid int) ([]int, error) { return []int{3210, 2}, nil }
	readProcStatForLiveness = func(pid int) (procStat, error) { return procStat{StartTimeTicks: 123456}, nil }
	isProcessAliveFunc = func(pid int) bool { return pid == 3210 }

	base := time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC)
	now := base
	manager := NewSessionManager(30 * time.Second)
	manager.now = func() time.Time { return now }

	manager.HandleMessage(Message{
		SessionID: "session-1",
		Event:     "waiting",
		Data: map[string]any{
			"_ppid":               float64(2),
			"_agent_pid_ns_inode": float64(4026533000),
			"_agent_start_time":   float64(123456),
			"_jai_jail":           true,
		},
	})
	_ = waitForSessionUpdate(t, manager.Updates())

	now = base.Add(31 * time.Second)
	manager.pruneExpired(now)

	sessions := manager.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("expected namespaced live session to remain, got %d sessions", len(sessions))
	}
}

func waitForSessionUpdate(t *testing.T, updates <-chan SessionUpdate) SessionUpdate {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session update")
		return SessionUpdate{}
	}
}
