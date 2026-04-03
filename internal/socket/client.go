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
