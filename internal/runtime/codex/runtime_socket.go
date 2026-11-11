package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
)

// NewUnixRuntime 连接 Unix domain socket，使用换行分隔的 JSON 消息；使用前须 Initialize。
// path 是 socket 文件路径。ctx 仅控制拨号，Runtime 拥有连接并通过 Close 释放。
func NewUnixRuntime(ctx context.Context, path string) (*Runtime, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("连接 Codex Unix socket: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), maxMessageBytes)
	return newRuntime(&streamTransport{
		ReadWriteCloser: conn,
		scanner:         scanner,
		encoder:         json.NewEncoder(conn),
	}), nil
}
