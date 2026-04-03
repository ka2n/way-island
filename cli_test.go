package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ka2n/way-island/internal/socket"
)

func TestRunHookSendsJSONToSocket(t *testing.T) {
	runtimeDir := t.TempDir()
	socketPath := filepath.Join(runtimeDir, "way-island.sock")
	received := make(chan socket.Message, 1)

	server := startHookTestServer(t, socketPath, socket.HandlerFunc(func(message socket.Message) {
		received <- message
	}))

	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	stdin := bytes.NewBufferString(`{"session_id":"stdin-session","hook_event_name":"PreToolUse","tool":"bash"}`)
	var stderr bytes.Buffer

	exitCode := run([]string{"hook"}, stdin, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}

	message := <-received
	if message.SessionID != "stdin-session" {
		t.Fatalf("unexpected session ID: %q", message.SessionID)
	}
	if message.Event != "tool_start" {
		t.Fatalf("unexpected event: %q", message.Event)
	}
	if message.Data["tool"] != "bash" {
		t.Fatalf("unexpected data.tool: %#v", message.Data["tool"])
	}

	shutdownHookTestServer(t, server)
}

func TestRunHookPrefersFlagSessionOverPayloadAndEnv(t *testing.T) {
	t.Setenv("WAY_ISLAND_SESSION_ID", "env-session")

	sessionID := resolveSessionID("flag-session", map[string]any{
		"session_id": "payload-session",
	})

	if sessionID != "flag-session" {
		t.Fatalf("unexpected resolved session ID: %q", sessionID)
	}
}

func TestRunHookFallsBackToEnvironmentSession(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "env-session")

	sessionID := resolveSessionID("", map[string]any{})
	if sessionID != "env-session" {
		t.Fatalf("unexpected resolved session ID: %q", sessionID)
	}
}

func TestRunHookSilentlySucceedsWithoutDaemon(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	var stderr bytes.Buffer
	stdin := bytes.NewBufferString(`{"session_id":"session-1","hook_event_name":"Notification"}`)
	exitCode := run([]string{"hook"}, stdin, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestRunHookIgnoresUnknownEvent(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	stdin := bytes.NewBufferString(`{"session_id":"session-1","hook_event_name":"UnknownEvent"}`)
	exitCode := run([]string{"hook"}, stdin, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for unknown event, got %d: %s", exitCode, stderr.String())
	}
}

func startHookTestServer(t *testing.T, path string, handler socket.Handler) *socket.Server {
	t.Helper()

	server, err := socket.NewServer(path, handler)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := server.Start(ctx); err != nil {
		if isHookSocketPermissionError(err) {
			t.Skipf("unix sockets are not permitted in this environment: %v", err)
		}
		t.Fatalf("start server: %v", err)
	}

	waitForHookTestSocket(t, path)
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
		_ = server.Wait()
	})

	return server
}

func shutdownHookTestServer(t *testing.T, server *socket.Server) {
	t.Helper()

	if err := server.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}
	if err := server.Wait(); err != nil {
		t.Fatalf("wait server: %v", err)
	}
}

func waitForHookTestSocket(t *testing.T, path string) {
	t.Helper()

	deadlineCtx, cancel := context.WithTimeout(context.Background(), socket.DefaultSessionTimeout)
	defer cancel()

	for deadlineCtx.Err() == nil {
		info, err := os.Stat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("socket was not created: %s", path)
}

func isHookSocketPermissionError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return errors.Is(opErr.Err, os.ErrPermission)
	}

	return errors.Is(err, os.ErrPermission)
}

func TestLoadHookPayloadDecodesJSON(t *testing.T) {
	t.Parallel()

	payload, err := loadHookPayload(bytes.NewBufferString(`{"session_id":"session-1","nested":{"ok":true}}`))
	if err != nil {
		t.Fatalf("load hook payload: %v", err)
	}

	rawNested, ok := payload["nested"]
	if !ok {
		t.Fatalf("expected nested payload")
	}
	data, err := json.Marshal(rawNested)
	if err != nil {
		t.Fatalf("marshal nested payload: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected nested payload: %s", data)
	}
}
