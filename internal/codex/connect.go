package codex

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type MessageTransport interface {
	ReadMessage(m *Message) error
	WriteMessage(m *Message) error
	Close() error
}

type websocketTransport struct {
	*websocket.Conn
}

func (t *websocketTransport) ReadMessage(m *Message) error {
	return t.ReadJSON(m)
}

func (t *websocketTransport) WriteMessage(m *Message) error {
	if err := t.SetWriteDeadline(time.Now().Add(requestTimeout)); err != nil {
		return err
	}
	return t.WriteJSON(m)
}

func NewUnixConnContext(ctx context.Context, sockPath string) (MessageTransport, error) {
	d := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
		},
	}
	c, resp, err := d.DialContext(ctx, "ws://localhost/", nil)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, fmt.Errorf("connect to codex socket: %w", err)
	}
	c.SetReadLimit(16 * 1024 * 1024)
	return &websocketTransport{c}, nil
}
