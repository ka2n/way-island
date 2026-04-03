package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const socketName = "way-island.sock"

var ErrAlreadyRunning = errors.New("way-island daemon is already running")

type Message struct {
	SessionID string         `json:"session_id"`
	Event     string         `json:"event"`
	Data      map[string]any `json:"data"`
}

type Handler interface {
	HandleMessage(Message)
}

type HandlerFunc func(Message)

func (f HandlerFunc) HandleMessage(message Message) {
	f(message)
}

type Server struct {
	path    string
	handler Handler

	mu       sync.Mutex
	listener net.Listener
	closeErr error
	waitCh   chan struct{}
	err      error
	once     sync.Once
}

func DefaultPath() (string, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set")
	}

	return filepath.Join(runtimeDir, socketName), nil
}

func NewServer(path string, handler Handler) (*Server, error) {
	if path == "" {
		return nil, errors.New("socket path is empty")
	}

	if handler == nil {
		handler = HandlerFunc(func(Message) {})
	}

	return &Server{
		path:    path,
		handler: handler,
		waitCh:  make(chan struct{}),
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	if err := prepareSocketPath(s.path); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("listen on unix socket: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	go s.serve(ctx)

	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener == nil {
		return nil
	}

	err := listener.Close()
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}

	return nil
}

func (s *Server) Wait() error {
	<-s.waitCh
	return s.err
}

func (s *Server) serve(ctx context.Context) {
	defer close(s.waitCh)
	defer s.cleanup()

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	for {
		conn, err := s.currentListener().Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}

			s.err = fmt.Errorf("accept connection: %w", err)
			return
		}

		go s.handleConn(conn)
	}
}

func (s *Server) currentListener() net.Listener {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	for {
		message, err := decodeMessage(decoder)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}

		s.handler.HandleMessage(message)
	}
}

func (s *Server) cleanup() {
	s.once.Do(func() {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.closeErr = err
			if s.err == nil {
				s.err = fmt.Errorf("remove socket file: %w", err)
			}
		}
	})
}

func prepareSocketPath(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat existing socket file: %w", err)
	}

	conn, err := net.Dial("unix", path)
	if err == nil {
		_ = conn.Close()
		return ErrAlreadyRunning
	}

	if !isStaleSocketError(err) {
		return fmt.Errorf("probe existing socket: %w", err)
	}

	return removeSocketFile(path)
}

func removeSocketFile(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("remove existing socket file: %w", err)
}

func isStaleSocketError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT)
}

func decodeMessage(decoder *json.Decoder) (Message, error) {
	var wire struct {
		SessionID string          `json:"session_id"`
		Event     string          `json:"event"`
		Data      json.RawMessage `json:"data"`
	}

	if err := decoder.Decode(&wire); err != nil {
		return Message{}, err
	}

	if strings.TrimSpace(wire.SessionID) == "" {
		return Message{}, errors.New("session_id is required")
	}

	if strings.TrimSpace(wire.Event) == "" {
		return Message{}, errors.New("event is required")
	}

	if !isJSONObject(wire.Data) {
		return Message{}, errors.New("data must be a JSON object")
	}

	data := make(map[string]any)
	if err := json.Unmarshal(wire.Data, &data); err != nil {
		return Message{}, fmt.Errorf("decode data: %w", err)
	}

	return Message{
		SessionID: wire.SessionID,
		Event:     wire.Event,
		Data:      data,
	}, nil
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}
