package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ka2n/way-island/internal/socket"
)

type hookSource string

const (
	hookSourceAuto   hookSource = "auto"
	hookSourceClaude hookSource = "claude"
	hookSourceCodex  hookSource = "codex"
)

// hookEventMapping maps Claude Code / Codex hook event names to internal event names.
var hookEventMapping = map[string]string{
	"PreToolUse":       "tool_start",
	"PostToolUse":      "tool_end",
	"Notification":     "waiting",
	"Stop":             "idle",
	"SessionStart":     "session_start",
	"SessionEnd":       "session_end",
	"UserPromptSubmit": "working",
}

func run(args []string, stdin io.Reader, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "hook":
			return runHook(args[1:], stdin, stderr)
		case "init":
			return runInit(args[1:], stderr)
		case "inspect":
			return runInspect(stderr)
		case "focus":
			return runFocus(args[1:], stderr)
		}
	}

	return runDaemon(stderr)
}

func runDaemon(stderr io.Writer) int {
	socketPath, err := socket.DefaultPath()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to resolve socket path: %v\n", err)
		return 1
	}

	debugf("daemon started pid=%d socket=%s", os.Getpid(), socketPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sessionManager := socket.NewSessionManager(socket.DefaultSessionTimeout)
	sessionManager.Start(ctx, socket.DefaultSessionTimeout)

	server, err := socket.NewServer(socketPath, sessionManager)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to create socket server: %v\n", err)
		return 1
	}

	if err := server.Start(ctx); err != nil {
		if errors.Is(err, socket.ErrAlreadyRunning) {
			_, _ = fmt.Fprintln(stderr, "way-island daemon is already running")
			return 1
		}
		_, _ = fmt.Fprintf(stderr, "failed to start socket server: %v\n", err)
		return 1
	}

	defer func() {
		stop()

		if err := server.Close(); err != nil {
			_, _ = fmt.Fprintf(stderr, "failed to close socket server: %v\n", err)
		}

		if serverErr := server.Wait(); serverErr != nil {
			_, _ = fmt.Fprintf(stderr, "socket server stopped with error: %v\n", serverErr)
		}
	}()

	merged := sessionManager.Updates()

	// Create the store (single source of truth) and wire it to the server for inspect/focus
	store := newOverlayModel()
	server.SetInspector(store)
	server.SetFocuser(newSessionFocuser(store))

	status := runUI(ctx, merged, store)

	return status
}

func runInspect(stderr io.Writer) int {
	socketPath, err := socket.DefaultPath()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to resolve socket path: %v\n", err)
		return 1
	}

	data, err := socket.Inspect(socketPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to inspect: %v\n", err)
		return 1
	}

	// Pretty-print JSON
	var pretty json.RawMessage
	if err := json.Unmarshal(data, &pretty); err == nil {
		formatted, err := json.MarshalIndent(pretty, "", "  ")
		if err == nil {
			data = formatted
		}
	}

	fmt.Println(string(data))
	return 0
}

func runFocus(args []string, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintf(stderr, "usage: way-island focus <session-id>\n")
		return 1
	}
	sessionID := args[0]

	socketPath, err := socket.DefaultPath()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to resolve socket path: %v\n", err)
		return 1
	}

	if err := socket.FocusSession(socketPath, sessionID); err != nil {
		_, _ = fmt.Fprintf(stderr, "focus failed: %v\n", err)
		return 1
	}

	return 0
}

func runHook(args []string, stdin io.Reader, stderr io.Writer) int {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	sessionID := fs.String("session", "", "Session identifier")
	claude := fs.Bool("claude", false, "Parse the hook payload as Claude Code")
	codex := fs.Bool("codex", false, "Parse the hook payload as Codex")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "usage: way-island hook [--session <id>] [--claude|--codex]\n")
		return 2
	}
	if *claude && *codex {
		_, _ = fmt.Fprintf(stderr, "hook source flags are mutually exclusive\n")
		return 2
	}

	payload, err := loadHookPayload(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to read hook payload: %v\n", err)
		return 2
	}

	source := resolveHookSource(*claude, *codex, payload)
	payload, hookEventName, err := parseHookPayload(source, payload)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	debugJSON("hook payload", payload)

	event, ok := hookEventMapping[hookEventName]
	debugf("hook_event_name=%q -> event=%q mapped=%v", hookEventName, event, ok)
	if !ok {
		// Unknown event type — silently ignore
		return 0
	}

	resolvedSessionID := resolveSessionID(source, *sessionID, payload)
	debugf("session_id=%q", resolvedSessionID)
	if strings.TrimSpace(resolvedSessionID) == "" {
		_, _ = fmt.Fprintf(stderr, "session ID is required\n")
		return 2
	}

	socketPath, err := socket.DefaultPath()
	if err != nil {
		return 0
	}

	// Attach parent PID for terminal jump (Phase 3)
	attachAgentMetadata(payload)
	payload["_hook_source"] = string(source)

	message := socket.Message{
		SessionID: resolvedSessionID,
		Event:     event,
		Data:      payload,
	}

	if err := socket.SendMessage(socketPath, message); err != nil {
		debugf("send error: %v", err)
		if isSilentHookError(err) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "failed to send hook event: %v\n", err)
		return 1
	}

	debugf("send ok: session=%q event=%q", resolvedSessionID, event)
	return 0
}

func attachAgentMetadata(payload map[string]any) {
	agentPID := os.Getppid()
	payload["_ppid"] = float64(agentPID)

	if nsInode, err := readPIDNamespaceInodeForPID(agentPID); err == nil && nsInode > 0 {
		payload["_agent_pid_ns_inode"] = float64(nsInode)
	}
	if stat, err := readProcStat(agentPID); err == nil {
		payload["_agent_start_time"] = float64(stat.StartTimeTicks)
		payload["_agent_tty_nr"] = float64(stat.TTYNr)
	}
	if tty := readTTYNameForPID(agentPID); tty != "" {
		payload["_agent_tty"] = tty
	}
	if tty := readTTYNameForPID(os.Getpid()); tty != "" {
		payload["_hook_tty"] = tty
	}
	if jaiJail := os.Getenv("JAI_JAIL"); strings.TrimSpace(jaiJail) != "" {
		payload["_jai_jail"] = true
	}
}

func loadHookPayload(stdin io.Reader) (map[string]any, error) {
	if stdin == nil || isInteractiveReader(stdin) {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.NewDecoder(stdin).Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}
		return nil, err
	}

	if payload == nil {
		payload = map[string]any{}
	}

	return payload, nil
}

func isInteractiveReader(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func resolveHookSource(forceClaude, forceCodex bool, payload map[string]any) hookSource {
	switch {
	case forceClaude:
		return hookSourceClaude
	case forceCodex:
		return hookSourceCodex
	case looksLikeCodexPayload(payload):
		return hookSourceCodex
	default:
		return hookSourceClaude
	}
}

func looksLikeCodexPayload(payload map[string]any) bool {
	if _, ok := payload["tool_input"]; ok {
		return true
	}
	if _, ok := payload["tool_name"]; ok {
		return true
	}
	if _, ok := payload["turn_id"]; ok {
		return true
	}
	return false
}

func parseHookPayload(source hookSource, payload map[string]any) (map[string]any, string, error) {
	switch source {
	case hookSourceClaude:
		return parseClaudeHookPayload(payload)
	case hookSourceCodex:
		return parseCodexHookPayload(payload)
	default:
		return nil, "", fmt.Errorf("unsupported hook source %q", source)
	}
}

func parseClaudeHookPayload(payload map[string]any) (map[string]any, string, error) {
	eventName := firstStringFromMap(payload, "hook_event_name", "hookEventName")
	if strings.TrimSpace(eventName) == "" {
		return nil, "", errors.New("hook_event_name is required")
	}
	return cloneHookPayload(payload), eventName, nil
}

func parseCodexHookPayload(payload map[string]any) (map[string]any, string, error) {
	eventName := firstStringFromMap(payload, "hook_event_name", "hookEventName")
	if strings.TrimSpace(eventName) == "" {
		return nil, "", errors.New("hook_event_name is required")
	}

	normalized := cloneHookPayload(payload)
	if _, ok := normalized["tool"]; !ok {
		if toolName := firstStringFromMap(normalized, "tool_name", "toolName"); strings.TrimSpace(toolName) != "" {
			normalized["tool"] = strings.ToLower(toolName)
		}
	}
	if command := firstNestedString(normalized, "tool_input", "command"); command != "" {
		normalized["command"] = command
	}

	return normalized, eventName, nil
}

func cloneHookPayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}

func resolveSessionID(source hookSource, flagValue string, payload map[string]any) string {
	candidates := []string{
		flagValue,
		firstStringFromMap(payload, "session_id", "sessionId"),
		os.Getenv("WAY_ISLAND_SESSION_ID"),
	}
	switch source {
	case hookSourceCodex:
		candidates = append(candidates, os.Getenv("CODEX_SESSION_ID"), os.Getenv("CLAUDE_CODE_SESSION_ID"), os.Getenv("CLAUDE_SESSION_ID"))
	default:
		candidates = append(candidates, os.Getenv("CLAUDE_CODE_SESSION_ID"), os.Getenv("CLAUDE_SESSION_ID"), os.Getenv("CODEX_SESSION_ID"))
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}

	return ""
}

func firstStringFromMap(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if ok && strings.TrimSpace(text) != "" {
			return text
		}
	}

	return ""
}

func firstNestedString(values map[string]any, parentKey string, childKeys ...string) string {
	parent, ok := values[parentKey].(map[string]any)
	if !ok {
		return ""
	}
	return firstStringFromMap(parent, childKeys...)
}

func isSilentHookError(err error) bool {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return errors.Is(opErr.Err, os.ErrNotExist) ||
			errors.Is(opErr.Err, syscall.ENOENT) ||
			errors.Is(opErr.Err, syscall.ECONNREFUSED) ||
			errors.Is(opErr.Err, os.ErrPermission) ||
			errors.Is(opErr.Err, syscall.EPERM)
	}

	return false
}
