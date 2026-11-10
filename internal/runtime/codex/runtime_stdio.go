package codex

import (
	"bufio"
	"encoding/json"
	"io"
)

type streamTransport struct {
	io.ReadWriteCloser
	scanner *bufio.Scanner
	encoder *json.Encoder
}

func (t *streamTransport) ReadMessage(m *Message) error {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return err
		}
		return io.EOF
	}
	return json.Unmarshal(t.scanner.Bytes(), m)
}

func (t *streamTransport) WriteMessage(m Message) error {
	return t.encoder.Encode(m)
}

// NewRuntime 创建 stdio Runtime，接管流的所有权；使用前须 Initialize。
func NewRuntime(conn io.ReadWriteCloser) *Runtime {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), maxMessageBytes)
	return newRuntime(&streamTransport{
		ReadWriteCloser: conn,
		scanner:         scanner,
		encoder:         json.NewEncoder(conn),
	})
}
