package codex

import (
	"context"
	"encoding/json"
	"io"

	base "github.com/hjhsamuel/magent/internal/runtime"
)

// Runtime 将 Codex 协议转换为平台统一接口，客户端仅在内部使用。
type Runtime struct {
	client *client
	events chan base.Event
}

// newRuntime 是两种传输共用的装配入口，不承担拨号和消息分帧。
func newRuntime(conn messageTransport) *Runtime {
	c := &client{
		conn:    conn,
		pending: make(map[string]chan Message),
		events:  make(chan Message, 256),
		done:    make(chan struct{}),
		writes:  make(chan writeRequest, 64),
	}
	r := &Runtime{
		client: c,
		events: make(chan base.Event),
	}

	go c.readLoop()
	go c.writeLoop()
	go r.forwardEvents()

	return r
}

func (r *Runtime) Initialize(ctx context.Context, info base.ClientInfo) (base.InitializeResponse, error) {
	out, err := r.client.Initialize(
		ctx,
		ClientInfo{
			Name:    info.Name,
			Title:   info.Title,
			Version: info.Version,
		},
	)
	return base.InitializeResponse{UserAgent: out.UserAgent}, err
}

// Events 必须持续消费；关闭连接后事件通道随转换协程退出而关闭。
func (r *Runtime) Events() <-chan base.Event {
	return r.events
}

func (r *Runtime) Done() <-chan struct{} {
	return r.client.done
}

func (r *Runtime) Err() error {
	return r.client.Err()
}

func (r *Runtime) Close() error {
	r.client.stop(io.ErrClosedPipe)
	return nil
}

func (r *Runtime) Respond(id json.RawMessage, result any, e *base.ResponseError) error {
	var rpcErr *RPCError
	if e != nil {
		rpcErr = &RPCError{
			Code:    e.Code,
			Message: e.Message,
			Data:    e.Data,
		}
	}
	return r.client.Respond(id, result, rpcErr)
}

// Call 是 Codex 专属扩展入口，不属于统一接口。
func (r *Runtime) Call(ctx context.Context, method string, params, result any) error {
	return r.client.Call(ctx, method, params, result)
}

func threadOptions(p base.SessionOptions) ThreadParams {
	return ThreadParams{
		Model:          p.Model,
		Cwd:            p.WorkingDirectory,
		ApprovalPolicy: p.ApprovalPolicy,
		Sandbox:        p.Sandbox,
	}
}

func (r *Runtime) sessionCall(ctx context.Context, method string, p any) (base.Session, error) {
	var out ThreadResponse
	err := r.client.Call(ctx, method, p, &out)
	return session(out.Thread), err
}

func (r *Runtime) StartSession(ctx context.Context, p base.SessionOptions) (base.Session, error) {
	return r.sessionCall(ctx, "thread/start", threadOptions(p))
}

func (r *Runtime) ResumeSession(ctx context.Context, p base.ResumeSessionParams) (base.Session, error) {
	return r.sessionCall(
		ctx,
		"thread/resume",
		ThreadResumeParams{
			ThreadID:     p.SessionID,
			ThreadParams: threadOptions(p.Options),
		},
	)
}

func (r *Runtime) ReadSession(ctx context.Context, p base.ReadSessionParams) (base.Session, error) {
	return r.sessionCall(
		ctx,
		"thread/read",
		ThreadReadParams{
			ThreadID:     p.SessionID,
			IncludeTurns: p.IncludeTurns,
		},
	)
}

func (r *Runtime) ListSessions(ctx context.Context, p base.ListSessionsParams) (base.SessionPage, error) {
	var out ThreadListResponse
	err := r.client.Call(
		ctx,
		"thread/list",
		ThreadListParams{
			Cursor:   p.Cursor,
			Limit:    p.Limit,
			Archived: p.Archived,
		},
		&out,
	)
	result := base.SessionPage{
		NextCursor: out.NextCursor,
		Sessions:   make([]base.Session, len(out.Data)),
	}
	for i, t := range out.Data {
		result.Sessions[i] = session(t)
	}
	return result, err
}

func (r *Runtime) ArchiveSession(ctx context.Context, id string) error {
	return r.client.Call(
		ctx,
		"thread/archive",
		ThreadIDParams{ThreadID: id},
		&EmptyResponse{},
	)
}

func (r *Runtime) UnarchiveSession(ctx context.Context, id string) (base.Session, error) {
	return r.sessionCall(ctx, "thread/unarchive", ThreadIDParams{ThreadID: id})
}

// StartTurn 仅表示服务器接受输入，最终状态通过 EventTurnCompleted 返回。
func (r *Runtime) StartTurn(ctx context.Context, p base.StartTurnParams) (base.Turn, error) {
	input := make([]Input, len(p.Input))
	for i, v := range p.Input {
		input[i] = Input{
			Type: v.Type,
			Text: v.Text,
			URL:  v.URL,
			Path: v.Path,
		}
	}

	var out TurnResponse
	err := r.client.Call(
		ctx,
		"turn/start",
		TurnStartParams{
			ThreadID: p.SessionID,
			Input:    input,
			Model:    p.Model,
		},
		&out,
	)
	return turn(out.Turn), err
}

func (r *Runtime) InterruptTurn(ctx context.Context, p base.InterruptTurnParams) error {
	return r.client.Call(
		ctx,
		"turn/interrupt",
		TurnInterruptParams{
			ThreadID: p.SessionID,
			TurnID:   p.TurnID,
		},
		&EmptyResponse{},
	)
}
