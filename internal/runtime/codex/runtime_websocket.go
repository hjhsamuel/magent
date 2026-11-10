package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type webSocketTransport struct {
	conn *websocket.Conn
}

func (t *webSocketTransport) ReadMessage(m *Message) error {
	kind, data, err := t.conn.ReadMessage()
	if err != nil {
		return err
	}

	if kind != websocket.TextMessage {
		return fmt.Errorf("app server 返回非文本 WebSocket 消息: %d", kind)
	}

	// 一条 WebSocket 消息必须对应一个 JSON 对象，不依赖尾部换行。
	return json.Unmarshal(data, m)
}

func (t *webSocketTransport) WriteMessage(m Message) error {
	return t.conn.WriteJSON(m)
}

func (t *webSocketTransport) Close() error {
	return t.conn.Close()
}

// NewWebSocketRuntime 连接 ws:// 或 wss:// app server，随后仍需 Initialize。
// headers 可携带认证信息；ctx 仅控制拨号和握手，不控制连接存续时间。
// Runtime 拥有连接，Close 会解除阻塞读写；不自动重连，避免重复提交轮次。
func NewWebSocketRuntime(ctx context.Context, endpoint string, headers http.Header) (*Runtime, error) {
	conn, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint, headers)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, fmt.Errorf("连接 Codex WebSocket: %w", err)
	}

	conn.SetReadLimit(maxMessageBytes)
	return newRuntime(&webSocketTransport{conn: conn}), nil
}
