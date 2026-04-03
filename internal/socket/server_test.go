package socket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServerReceivesJSONMessages(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	received := make(chan Message, 1)

	server := startTestServer(t, socketPath, HandlerFunc(func(message Message) {
		received <- message
	}))

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer conn.Close()

	payload := map[string]any{
		"session_id": "session-1",
		"event":      "working",
		"data": map[string]any{
			"tool": "bash",
		},
	}

	if err := json.NewEncoder(conn).Encode(payload); err != nil {
		t.Fatalf("encode message: %v", err)
	}

	select {
	case message := <-received:
		if message.SessionID != "session-1" {
			t.Fatalf("unexpected session_id: %q", message.SessionID)
		}
		if message.Event != "working" {
			t.Fatalf("unexpected event: %q", message.Event)
		}
		if got := message.Data["tool"]; got != "bash" {
			t.Fatalf("unexpected data.tool: %#v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	shutdownAndWait(t, server)
}

func TestServerSurvivesInvalidMessages(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	received := make(chan Message, 1)

	server := startTestServer(t, socketPath, HandlerFunc(func(message Message) {
		received <- message
	}))

	invalidConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial invalid socket connection: %v", err)
	}

	if _, err := invalidConn.Write([]byte("{invalid json\n")); err != nil {
		t.Fatalf("write invalid message: %v", err)
	}
	_ = invalidConn.Close()

	validConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial valid socket connection: %v", err)
	}
	defer validConn.Close()

	if err := json.NewEncoder(validConn).Encode(map[string]any{
		"session_id": "session-2",
		"event":      "idle",
		"data":       map[string]any{},
	}); err != nil {
		t.Fatalf("encode valid message: %v", err)
	}

	select {
	case message := <-received:
		if message.SessionID != "session-2" {
			t.Fatalf("unexpected session_id after invalid message: %q", message.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for valid message after invalid input")
	}

	shutdownAndWait(t, server)
}

func TestServerRecreatesExistingSocketFile(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	if err := os.WriteFile(socketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale socket file: %v", err)
	}

	server := startTestServer(t, socketPath, nil)

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("stat socket file: %v", err)
	}

	if info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("expected unix socket file, got mode %v", info.Mode())
	}

	shutdownAndWait(t, server)
}

func TestServerRejectsDuplicateDaemon(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	running := startTestServer(t, socketPath, nil)

	duplicate, err := NewServer(socketPath, nil)
	if err != nil {
		t.Fatalf("new duplicate server: %v", err)
	}

	err = duplicate.Start(context.Background())
	if isSocketPermissionError(err) {
		t.Skipf("unix sockets are not permitted in this environment: %v", err)
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}

	shutdownAndWait(t, running)
}

func TestServerRemovesSocketOnShutdown(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	server := startTestServer(t, socketPath, nil)

	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("expected socket file to exist: %v", err)
	}

	shutdownAndWait(t, server)

	if _, err := os.Stat(socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected socket file to be removed, got err=%v", err)
	}
}

func TestServerRemovesStaleSocketWhenProbeFails(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		if isSocketPermissionError(err) {
			t.Skipf("unix sockets are not permitted in this environment: %v", err)
		}
		t.Fatalf("listen stale socket: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close stale listener: %v", err)
	}

	server := startTestServer(t, socketPath, nil)
	shutdownAndWait(t, server)
}

func startTestServer(t *testing.T, path string, handler Handler) *Server {
	t.Helper()

	server, err := NewServer(path, handler)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := server.Start(ctx); err != nil {
		if isSocketPermissionError(err) {
			t.Skipf("unix sockets are not permitted in this environment: %v", err)
		}
		t.Fatalf("start server: %v", err)
	}

	waitForSocket(t, path)
	t.Cleanup(func() {
		cancel()
		_ = server.Close()
		_ = server.Wait()
	})

	return server
}

func shutdownAndWait(t *testing.T, server *Server) {
	t.Helper()

	if err := server.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}

	if err := server.Wait(); err != nil {
		t.Fatalf("wait for server: %v", err)
	}
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("socket was not created: %s", path)
}

func isSocketPermissionError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return errors.Is(opErr.Err, os.ErrPermission)
	}

	return errors.Is(err, os.ErrPermission)
}
