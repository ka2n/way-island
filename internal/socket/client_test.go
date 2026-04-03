package socket

import (
	"path/filepath"
	"testing"
)

func TestSendMessageWritesJSONToServer(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "way-island.sock")
	received := make(chan Message, 1)

	server := startTestServer(t, socketPath, HandlerFunc(func(message Message) {
		received <- message
	}))

	err := SendMessage(socketPath, Message{
		SessionID: "session-1",
		Event:     "response",
		Data: map[string]any{
			"text": "done",
		},
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	message := <-received
	if message.SessionID != "session-1" {
		t.Fatalf("unexpected session ID: %q", message.SessionID)
	}
	if message.Event != "response" {
		t.Fatalf("unexpected event: %q", message.Event)
	}
	if message.Data["text"] != "done" {
		t.Fatalf("unexpected data.text: %#v", message.Data["text"])
	}

	shutdownAndWait(t, server)
}
