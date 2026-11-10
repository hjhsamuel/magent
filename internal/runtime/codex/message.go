package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strconv"
	"sync"
)

// client 管理单条连接。调用方提供已连接的标准输入输出流，并负责启动服务器。
// Events 必须持续消费；队列溢出会关闭连接，避免静默丢失完成事件。
type client struct {
	conn    messageTransport
	writes  chan writeRequest
	mu      sync.Mutex
	next    uint64
	pending map[string]chan Message
	events  chan Message
	done    chan struct{}
	once    sync.Once
	err     error
}

// Err 返回连接终止原因。
func (c *client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *client) stop(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		close(c.done)
		c.mu.Unlock()
		_ = c.conn.Close()
	})
}

// writeRequest 使用缓冲完成通道，调用方取消后不会阻塞写协程。
type writeRequest struct {
	ctx     context.Context
	message Message
	result  chan error
}

// writeLoop 复用编码器和单个写协程。有界队列避免慢连接累积大量协程。
func (c *client) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case req := <-c.writes:
			if err := req.ctx.Err(); err != nil {
				req.result <- err
				continue
			}

			select {
			case <-c.done:
				return
			default:
			}

			err := c.conn.WriteMessage(req.message)
			req.result <- err
			if err != nil {
				c.stop(err)
				return
			}
		}
	}
}

func (c *client) writeContext(ctx context.Context, m Message) error {
	req := writeRequest{
		ctx:     ctx,
		message: m,
		result:  make(chan error, 1),
	}

	select {
	case c.writes <- req:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.Err()
	}

	select {
	case err := <-req.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.Err()
	}
}

func (c *client) readLoop() {
	defer close(c.events)

	for {
		var m Message
		if err := c.conn.ReadMessage(&m); err != nil {
			c.stop(fmt.Errorf("读取 app server 消息: %w", err))
			return
		}

		if m.Method == "" && len(m.ID) > 0 {
			c.mu.Lock()
			ch := c.pending[string(m.ID)]
			c.mu.Unlock()
			if ch != nil {
				select {
				case ch <- m:
				default:
				}
			}
			continue
		}

		select {
		case c.events <- m:
		case <-c.done:
			return
		default:
			c.stop(errors.New("codex 事件队列已满"))
			return
		}
	}
}

// Call 发送请求并按 ID 匹配响应。取消只停止本地等待；中断推理须调用 InterruptTurn。
func (c *client) Call(ctx context.Context, method string, params, result any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.next++
	id := strconv.FormatUint(c.next, 10)
	ch := make(chan Message, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err = c.writeContext(
		ctx,
		Message{
			ID:     json.RawMessage(id),
			Method: method,
			Params: data,
		},
	); err != nil {
		return err
	}

	select {
	case m := <-ch:
		if m.Error != nil {
			return m.Error
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(m.Result, result)
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return c.Err()
	}
}

// Respond 回复服务端的审批或工具请求；不会自动批准操作。
func (c *client) Respond(id json.RawMessage, result any, rpcErr *RPCError) error {
	var data json.RawMessage
	if rpcErr == nil {
		var err error
		data, err = json.Marshal(result)
		if err != nil {
			return err
		}
	}
	return c.writeContext(
		context.Background(),
		Message{
			ID:     id,
			Result: data,
			Error:  rpcErr,
		},
	)
}

// Initialize 执行协议握手，每条连接仅调用一次。
func (c *client) Initialize(ctx context.Context, info ClientInfo) (InitializeResponse, error) {
	var out InitializeResponse
	err := c.Call(ctx, "initialize", InitializeParams{ClientInfo: info}, &out)
	if err == nil {
		err = c.writeContext(ctx, Message{Method: "initialized"})
	}
	return out, err
}
