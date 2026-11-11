package codex

const maxMessageBytes = 16 * 1024 * 1024

// messageTransport 隔离消息边界：stdio 和 Unix socket 使用换行，WebSocket 使用消息帧。
// 客户端保证单读单写；Close 必须能解除正在阻塞的读写。
type messageTransport interface {
	ReadMessage(*Message) error
	WriteMessage(Message) error
	Close() error
}
