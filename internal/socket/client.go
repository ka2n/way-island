package socket

import (
	"encoding/json"
	"fmt"
	"net"
)

func SendMessage(path string, message Message) error {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("dial unix socket: %w", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(message); err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	return nil
}

// FocusSession sends a focus request for the given session and returns any error.
func FocusSession(path string, sessionID string) error {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("dial unix socket: %w", err)
	}
	defer conn.Close()

	msg := Message{SessionID: sessionID, Event: "_focus", Data: map[string]any{}}
	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return fmt.Errorf("send focus request: %w", err)
	}

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("read focus response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// Inspect sends an inspect request and returns the raw JSON response.
func Inspect(path string) ([]byte, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("dial unix socket: %w", err)
	}
	defer conn.Close()

	msg := Message{SessionID: "_inspect", Event: "_inspect", Data: map[string]any{}}
	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return nil, fmt.Errorf("send inspect request: %w", err)
	}

	var result json.RawMessage
	if err := json.NewDecoder(conn).Decode(&result); err != nil {
		return nil, fmt.Errorf("read inspect response: %w", err)
	}

	return result, nil
}
