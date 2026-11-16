package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

type MessageTransport interface {
	ReadMessage(m *Message) error
	WriteMessage(m *Message) error
	Close() error
}

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

func (t *streamTransport) WriteMessage(m *Message) error {
	return t.encoder.Encode(m)
}

func NewUnixConn(sockPath string) (MessageTransport, error) {
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("connect to codex socket: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)

	return &streamTransport{
		ReadWriteCloser: conn,
		scanner:         scanner,
		encoder:         json.NewEncoder(conn),
	}, nil
}
