package socket

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const DefaultSessionTimeout = 5 * time.Minute
const DefaultTranscriptReadDelay = 500 * time.Millisecond
const maxLastUserMessageRunes = 140

type SessionState string

const (
	SessionStateIdle        SessionState = "idle"
	SessionStateWorking     SessionState = "working"
	SessionStateToolRunning SessionState = "tool_running"
	SessionStateWaiting     SessionState = "waiting"
)

type Session struct {
	ID                     string
	DisplayName            string
	State                  SessionState
	CurrentTool            string
	CurrentAction          string
	CurrentToolFailed      bool
	LastUserMessage        string
	LastAssistantMessage   string
	ParentSessionID        string
	IsSubagent             bool
	AgentNickname          string
	Subagents              []SubagentSummary
	HookSource             string
	LastEventAt            time.Time
	AgentPID               int // PID of the agent process as seen from the hook namespace
	AgentPIDNamespaceInode uint64
	AgentStartTimeTicks    uint64
	AgentTTY               string
	AgentTTYNr             int64
	HookTTY                string
	AgentInJail            bool
	TermProgram            string
}

type SessionUpdateType string

const (
	SessionUpdateUpsert  SessionUpdateType = "upsert"
	SessionUpdateTimeout SessionUpdateType = "timeout"
)

type SessionUpdate struct {
	Type    SessionUpdateType
	Session Session
	Reason  string // human-readable reason for the update (e.g. hook event name)
}

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]Session
	monitors map[string]sessionMonitorEntry
	timeout  time.Duration
	now      func() time.Time
	updates  chan SessionUpdate

	// transcriptReadDelay is the delay before reading the Claude transcript
	// after a Stop hook fires. The Stop hook fires before Claude Code writes
	// the current response to the transcript file, so we need to wait.
	transcriptReadDelay time.Duration
}

type sessionMonitorEntry struct {
	identity processIdentity
	monitor  sessionProcessMonitor
}

type processIdentity struct {
	AgentPID               int
	AgentPIDNamespaceInode uint64
	AgentStartTimeTicks    uint64
}

type procStat struct {
	StartTimeTicks uint64
}

var (
	isProcessAliveFunc = func(pid int) bool {
		return syscall.Kill(pid, 0) == nil
	}
	listProcPIDsForLiveness = func() ([]int, error) {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return nil, err
		}

		pids := make([]int, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			pids = append(pids, pid)
		}
		return pids, nil
	}
	readPIDNamespaceInodeForLiveness = func(pid int) (uint64, error) {
		info, err := os.Stat(fmt.Sprintf("/proc/%d/ns/pid", pid))
		if err != nil {
			return 0, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return 0, fmt.Errorf("unexpected stat type for pid namespace %d", pid)
		}
		return stat.Ino, nil
	}
	readNSPIDsForLiveness = func(pid int) ([]int, error) {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			return nil, err
		}

		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "NSpid:") {
				continue
			}
			raw := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "NSpid:")))
			pids := make([]int, 0, len(raw))
			for _, value := range raw {
				pidValue, err := strconv.Atoi(value)
				if err != nil {
					return nil, err
				}
				pids = append(pids, pidValue)
			}
			return pids, nil
		}

		return nil, fmt.Errorf("NSpid not found for pid %d", pid)
	}
	readProcStatForLiveness = func(pid int) (procStat, error) {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return procStat{}, err
		}

		text := strings.TrimSpace(string(data))
		closeIdx := strings.LastIndex(text, ")")
		if closeIdx == -1 || closeIdx+2 >= len(text) {
			return procStat{}, fmt.Errorf("unexpected stat format for pid %d", pid)
		}
		fields := strings.Fields(text[closeIdx+2:])
		if len(fields) < 20 {
			return procStat{}, fmt.Errorf("unexpected stat field count for pid %d", pid)
		}
		startTimeTicks, err := strconv.ParseUint(fields[19], 10, 64)
		if err != nil {
			return procStat{}, err
		}
		return procStat{StartTimeTicks: startTimeTicks}, nil
	}
)

func NewSessionManager(timeout time.Duration) *SessionManager {
	if timeout <= 0 {
		timeout = DefaultSessionTimeout
	}

	return &SessionManager{
		sessions:            make(map[string]Session),
		monitors:            make(map[string]sessionMonitorEntry),
		timeout:             timeout,
		now:                 time.Now,
		updates:             make(chan SessionUpdate, 32),
		transcriptReadDelay: DefaultTranscriptReadDelay,
	}
}

func (m *SessionManager) Updates() <-chan SessionUpdate {
	return m.updates
}

func (m *SessionManager) HandleMessage(message Message) {
	if message.Event == "session_end" {
		m.removeSession(message.SessionID, "hook:session_end")
		return
	}

	state, ok := sessionStateFromEvent(message.Event)
	if !ok {
		return
	}

	m.mu.Lock()
	existing := m.sessions[message.SessionID]
	hookSource := resolveString(existing.HookSource, message.Data, "_hook_source")
	isSubagent := existing.IsSubagent
	parentSessionID := existing.ParentSessionID
	agentNickname := existing.AgentNickname
	subagents := existing.Subagents
	session := Session{
		ID:                     message.SessionID,
		DisplayName:            resolveDisplayName(existing.DisplayName, message.Data),
		State:                  state,
		CurrentTool:            resolveCurrentTool(existing.CurrentTool, message.Event, message.Data),
		CurrentAction:          resolveCurrentAction(existing.CurrentAction, message.Event, message.Data),
		CurrentToolFailed:      resolveCurrentToolFailed(existing.CurrentToolFailed, message.Event),
		LastUserMessage:      resolveLastUserMessage(existing.LastUserMessage, message.Data),
		LastAssistantMessage: existing.LastAssistantMessage,
		ParentSessionID:        parentSessionID,
		IsSubagent:             isSubagent,
		AgentNickname:          agentNickname,
		Subagents:              subagents,
		HookSource:             hookSource,
		LastEventAt:            m.now(),
		AgentPID:               resolveAgentPID(existing.AgentPID, message.Data),
		AgentPIDNamespaceInode: resolveUint64(existing.AgentPIDNamespaceInode, message.Data, "_agent_pid_ns_inode"),
		AgentStartTimeTicks:    resolveUint64(existing.AgentStartTimeTicks, message.Data, "_agent_start_time"),
		AgentTTY:               resolveString(existing.AgentTTY, message.Data, "_agent_tty"),
		AgentTTYNr:             resolveInt64(existing.AgentTTYNr, message.Data, "_agent_tty_nr"),
		HookTTY:                resolveString(existing.HookTTY, message.Data, "_hook_tty"),
		AgentInJail:            resolveBool(existing.AgentInJail, message.Data, "_jai_jail"),
		TermProgram:            resolveString(existing.TermProgram, message.Data, "_term_program"),
	}
	if hookSource == "codex" && shouldEnrichCodexSessionMetadata(existing, session) {
		if metadata, ok := readCodexSessionMetadataFunc(message.SessionID, message.Data); ok {
			if session.ParentSessionID == "" {
				session.ParentSessionID = metadata.ParentSessionID
			}
			if session.AgentNickname == "" {
				session.AgentNickname = metadata.AgentNickname
			}
			if metadata.IsSubagent {
				session.IsSubagent = true
			}
		}
	}
	if hookSource == "claude" {
		if metadata, ok := readClaudeSessionMetadataFunc(message.SessionID, message.Data); ok {
			if len(metadata.Subagents) > 0 {
				session.Subagents = metadata.Subagents
			}
		}
	}
	if message.Event == string(SessionStateIdle) && !session.IsSubagent {
		switch hookSource {
		case "claude":
			// The Stop hook fires before Claude Code writes the current
			// response to the transcript file. Defer the read so the
			// transcript has time to be flushed.
			go m.deferredReadLastAssistantMessage(session.ID, message.Data)
		case "codex":
			if text, ok := readCodexLastAssistantMessageFunc(message.Data); ok {
				session.LastAssistantMessage = text
			}
		}
	}
	session.State = adjustSessionState(message.Event, session.State, session.IsSubagent)
	m.sessions[message.SessionID] = session
	m.mu.Unlock()

	m.notify(SessionUpdate{
		Type:    SessionUpdateUpsert,
		Session: session,
		Reason:  "hook:" + message.Event,
	})

	m.ensureSessionMonitor(session)
}

func adjustSessionState(event string, state SessionState, isSubagent bool) SessionState {
	if event == string(SessionStateIdle) && state == SessionStateIdle && !isSubagent {
		return SessionStateWaiting
	}
	return state
}

func shouldEnrichCodexSessionMetadata(existing Session, current Session) bool {
	if current.IsSubagent || strings.TrimSpace(current.ParentSessionID) != "" {
		return false
	}
	if strings.TrimSpace(current.AgentNickname) != "" && strings.TrimSpace(existing.ParentSessionID) != "" {
		return false
	}
	return true
}


func (m *SessionManager) deferredReadLastAssistantMessage(sessionID string, data map[string]any) {
	time.Sleep(m.transcriptReadDelay)

	text, ok := readClaudeLastAssistantMessageFunc(data)
	if !ok {
		return
	}

	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists || session.LastAssistantMessage == text {
		m.mu.Unlock()
		return
	}
	session.LastAssistantMessage = text
	m.sessions[sessionID] = session
	m.mu.Unlock()

	m.notify(SessionUpdate{
		Type:    SessionUpdateUpsert,
		Session: session,
		Reason:  "deferred:last_assistant_message",
	})
}

func (m *SessionManager) removeSession(sessionID string, reason string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	monitor, monitorOK := m.monitors[sessionID]
	if monitorOK {
		delete(m.monitors, sessionID)
	}
	m.mu.Unlock()

	if monitorOK {
		_ = monitor.monitor.Close()
	}

	if !ok {
		return
	}

	m.notify(SessionUpdate{
		Type:    SessionUpdateTimeout,
		Session: session,
		Reason:  reason,
	})
}

func (m *SessionManager) ensureSessionMonitor(session Session) {
	identity, ok := processIdentityFromSession(session)
	if !ok {
		m.clearSessionMonitor(session.ID)
		return
	}

	m.mu.Lock()
	existing, exists := m.monitors[session.ID]
	m.mu.Unlock()
	if exists && existing.identity == identity {
		return
	}

	monitor, err := newSessionProcessMonitor(session, func() {
		m.handleSessionProcessExit(session.ID, identity)
	})
	if err != nil {
		debugf("session monitor unavailable session_id=%s pid=%d err=%v", session.ID, session.AgentPID, err)
		m.clearSessionMonitor(session.ID)
		return
	}

	m.mu.Lock()
	replaced, hadReplaced := m.monitors[session.ID]
	m.monitors[session.ID] = sessionMonitorEntry{
		identity: identity,
		monitor:  monitor,
	}
	m.mu.Unlock()

	if hadReplaced {
		_ = replaced.monitor.Close()
	}
}

func (m *SessionManager) clearSessionMonitor(sessionID string) {
	m.mu.Lock()
	entry, ok := m.monitors[sessionID]
	if ok {
		delete(m.monitors, sessionID)
	}
	m.mu.Unlock()
	if ok {
		_ = entry.monitor.Close()
	}
}

func (m *SessionManager) handleSessionProcessExit(sessionID string, identity processIdentity) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return
	}
	currentIdentity, currentOK := processIdentityFromSession(session)
	if !currentOK || currentIdentity != identity {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	m.removeSession(sessionID, "pidfd:process_exit")
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
		if session.AgentPID > 0 && isSessionProcessAlive(session) {
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
			Reason:  "timeout",
		})
	}
}

func (m *SessionManager) notify(update SessionUpdate) {
	select {
	case m.updates <- update:
	default:
		debugf("notify: dropped update session_id=%s event=%s (channel full)", update.Session.ID, update.Reason)
	}
}

// resolveDisplayName returns a human-readable name for the session.
// It prefers the cwd basename from the hook payload; falls back to the existing name.
func resolveDisplayName(existing string, data map[string]any) string {
	if cwd, ok := data["cwd"].(string); ok && cwd != "" {
		if name := filepath.Base(cwd); name != "" && name != "." {
			return name
		}
	}
	return existing
}

// resolveAgentPID extracts the agent PID from hook payload.
// The _ppid field is set by the hook process using os.Getppid().
func resolveAgentPID(existing int, data map[string]any) int {
	if ppid, ok := data["_ppid"].(float64); ok && ppid > 0 {
		return int(ppid)
	}
	return existing
}

func resolveUint64(existing uint64, data map[string]any, key string) uint64 {
	if value, ok := data[key].(float64); ok && value > 0 {
		return uint64(value)
	}
	return existing
}

func resolveInt64(existing int64, data map[string]any, key string) int64 {
	if value, ok := data[key].(float64); ok {
		return int64(value)
	}
	return existing
}

func resolveString(existing string, data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok && value != "" {
			return value
		}
	}
	return existing
}

func resolveBool(existing bool, data map[string]any, key string) bool {
	if value, ok := data[key].(bool); ok {
		return value
	}
	return existing
}

func resolveCurrentAction(existing string, event string, data map[string]any) string {
	switch event {
	case "tool_start", string(SessionStateToolRunning):
		// Codex currently does not expose a distinct approval-request hook/event,
		// so PreToolUse is the closest external signal for "about to block on
		// approval". Track upstream: openai/codex#15311, #16301, #16484.
		toolName := strings.TrimSpace(resolveString("", data, "tool_name", "tool"))
		command := strings.TrimSpace(resolveString("", data, "command"))
		detail := resolveToolDetail(data)
		switch {
		case toolName != "" && command != "":
			return toolName + ": " + command
		case toolName != "" && detail != "":
			return toolName + ": " + detail
		case toolName != "":
			return toolName
		case command != "":
			return command
		default:
			return existing
		}
	case "compacting":
		return "Compacting context…"
	case "tool_end", "tool_end_failure", "permission_denied",
		string(SessionStateWorking), string(SessionStateWaiting), string(SessionStateIdle),
		"session_start", "response", "subagent_start", "subagent_stop":
		return ""
	default:
		return existing
	}
}

func resolveCurrentToolFailed(existing bool, event string) bool {
	switch event {
	case "tool_end_failure":
		return true
	case "tool_start", "tool_end", "permission_denied",
		string(SessionStateWorking), string(SessionStateWaiting), string(SessionStateIdle), "session_start":
		return false
	default:
		return existing
	}
}

// resolveToolDetail extracts a human-readable detail string from tool_input.
// It checks file_path (basename only), pattern, and prompt (truncated to 40 chars).
func resolveToolDetail(data map[string]any) string {
	if filePath := firstNestedString(data, "tool_input", "file_path"); filePath != "" {
		base := filepath.Base(filePath)
		if base != "" && base != "." {
			return base
		}
	}
	if pattern := firstNestedString(data, "tool_input", "pattern"); pattern != "" {
		return pattern
	}
	if prompt := firstNestedString(data, "tool_input", "prompt"); prompt != "" {
		const maxPromptLen = 40
		runes := []rune(prompt)
		if len(runes) > maxPromptLen {
			return string(runes[:maxPromptLen-1]) + "…"
		}
		return prompt
	}
	return ""
}

func resolveCurrentTool(existing string, event string, data map[string]any) string {
	switch event {
	case "tool_start", string(SessionStateToolRunning):
		return normalizeToolName(resolveString(existing, data, "tool", "tool_name"))
	case "tool_end", "tool_end_failure", "permission_denied", "compacting",
		string(SessionStateWorking), string(SessionStateWaiting), string(SessionStateIdle),
		"session_start", "response", "subagent_start", "subagent_stop":
		return ""
	default:
		return existing
	}
}

func resolveLastUserMessage(existing string, data map[string]any) string {
	if !isUserPromptSubmit(data) {
		return existing
	}

	text := strings.TrimSpace(firstNonEmptyString(
		firstString(data, "prompt", "user_prompt", "userPrompt", "text"),
		firstNestedString(data, "prompt", "text", "content"),
		firstNestedString(data, "message", "text", "content"),
	))
	if text == "" {
		return existing
	}

	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > maxLastUserMessageRunes {
		runes := []rune(text)
		text = string(runes[:maxLastUserMessageRunes-3]) + "..."
	}
	return text
}

func isUserPromptSubmit(data map[string]any) bool {
	eventName := strings.TrimSpace(resolveString("", data, "hook_event_name", "hookEventName"))
	return eventName == "UserPromptSubmit"
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNestedString(data map[string]any, parent string, keys ...string) string {
	value, ok := data[parent].(map[string]any)
	if !ok {
		return ""
	}
	return firstString(value, keys...)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isProcessAlive(pid int) bool {
	return isProcessAliveFunc(pid)
}

func isSessionProcessAlive(session Session) bool {
	if session.AgentPID <= 0 {
		return false
	}

	resolver := newLivenessHostPIDResolver()
	if hostPID, ok := resolver.Resolve(session); ok {
		return isProcessAlive(hostPID)
	}

	if session.AgentInJail || session.AgentPIDNamespaceInode > 0 && session.AgentPID < 100 {
		return false
	}

	return isProcessAlive(session.AgentPID)
}

func newLivenessHostPIDResolver() HostPIDResolver {
	return HostPIDResolver{
		ReadCurrentPIDNSInode: func() (uint64, error) {
			return readPIDNamespaceInodeForLiveness(os.Getpid())
		},
		ReadPIDNamespaceInode: readPIDNamespaceInodeForLiveness,
		ReadNamespacedPIDs:    readNSPIDsForLiveness,
		ReadStartTimeTicks: func(pid int) (uint64, error) {
			stat, err := readProcStatForLiveness(pid)
			if err != nil {
				return 0, err
			}
			return stat.StartTimeTicks, nil
		},
		ListPIDs: listProcPIDsForLiveness,
	}
}

func processIdentityFromSession(session Session) (processIdentity, bool) {
	if session.AgentPID <= 0 {
		return processIdentity{}, false
	}
	return processIdentity{
		AgentPID:               session.AgentPID,
		AgentPIDNamespaceInode: session.AgentPIDNamespaceInode,
		AgentStartTimeTicks:    session.AgentStartTimeTicks,
	}, true
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
	case "tool_end_failure":
		return SessionStateWorking, true
	case "permission_denied":
		return SessionStateWorking, true
	case "subagent_start":
		return SessionStateWorking, true
	case "subagent_stop":
		return SessionStateWorking, true
	case "compacting":
		return SessionStateWorking, true
	case "response":
		return SessionStateWaiting, true
	default:
		return "", false
	}
}
