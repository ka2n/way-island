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

// hookEventMapping maps Claude Code hook_event_name values to internal event names.
var hookEventMapping = map[string]string{
	"PreToolUse":        "tool_start",
	"PostToolUse":       "tool_end",
	"Notification":      "waiting",
	"Stop":              "idle",
	"SessionStart":      "session_start",
	"SessionEnd":        "session_end",
	"UserPromptSubmit":  "working",
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

	// Create the store (single source of truth) and wire it to the server for inspect
	store := newOverlayModel()
	server.SetInspector(store)

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


func runHook(args []string, stdin io.Reader, stderr io.Writer) int {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	sessionID := fs.String("session", "", "Session identifier")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "usage: way-island hook [--session <id>]\n")
		return 2
	}

	payload, err := loadHookPayload(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to read hook payload: %v\n", err)
		return 2
	}

	debugJSON("hook payload", payload)

	hookEventName, _ := payload["hook_event_name"].(string)
	event, ok := hookEventMapping[hookEventName]
	debugf("hook_event_name=%q -> event=%q mapped=%v", hookEventName, event, ok)
	if !ok {
		// Unknown event type — silently ignore
		return 0
	}

	resolvedSessionID := resolveSessionID(*sessionID, payload)
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
	payload["_ppid"] = float64(os.Getppid())

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

func resolveSessionID(flagValue string, payload map[string]any) string {
	for _, candidate := range []string{
		flagValue,
		firstStringFromMap(payload, "session_id", "sessionId"),
		os.Getenv("WAY_ISLAND_SESSION_ID"),
		os.Getenv("CLAUDE_CODE_SESSION_ID"),
		os.Getenv("CLAUDE_SESSION_ID"),
	} {
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
