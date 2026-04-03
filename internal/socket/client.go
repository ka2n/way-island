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
