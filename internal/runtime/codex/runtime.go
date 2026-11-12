package codex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	base "github.com/hjhsamuel/magent/internal/runtime"
)

// Runtime 将 Codex 协议转换为平台统一接口，客户端仅在内部使用。
type Runtime struct {
	client       *client
	events       chan base.Event
	mu           sync.Mutex
	sessions     map[string]struct{}
	closing      chan struct{}
	closeOnce    sync.Once
	closeErr     error
	sessionCalls sync.WaitGroup
}

// newRuntime 是两种传输共用的装配入口，不承担拨号和消息分帧。
func newRuntime(conn messageTransport) *Runtime {
	c := &client{
		conn:    conn,
		pending: make(map[string]chan Message),
		events:  make(chan Message, 1024),
		done:    make(chan struct{}),
		writes:  make(chan writeRequest, 64),
	}
	r := &Runtime{
		client:   c,
		events:   make(chan base.Event),
		sessions: make(map[string]struct{}),
		closing:  make(chan struct{}),
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
	if err != nil {
		return base.InitializeResponse{}, err
	}

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
	r.closeOnce.Do(func() {
		r.mu.Lock()
		close(r.closing)
		r.mu.Unlock()
		r.sessionCalls.Wait()
		defer r.client.stop(io.ErrClosedPipe)
		select {
		case <-r.Done():
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		r.mu.Lock()
		ids := make([]string, 0, len(r.sessions))
		for id := range r.sessions {
			ids = append(ids, id)
		}
		r.sessions = nil
		r.mu.Unlock()

		for _, id := range ids {
			var out ThreadUnsubscribeResponse
			if err := r.client.Call(ctx, "thread/unsubscribe", ThreadIDParams{ThreadID: id}, &out); err != nil {
				r.closeErr = errors.Join(r.closeErr, err)
			}
		}
	})
	return r.closeErr
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
	if method != "thread/start" && method != "thread/resume" {
		var out ThreadResponse
		err := r.client.Call(ctx, method, p, &out)
		return session(out.Thread), err
	}
	r.mu.Lock()
	select {
	case <-r.closing:
		r.mu.Unlock()
		return base.Session{}, io.ErrClosedPipe
	default:
	}
	r.sessionCalls.Add(1)
	r.mu.Unlock()
	defer r.sessionCalls.Done()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-r.closing:
			cancel()
		case <-ctx.Done():
		}
	}()
	var out ThreadResponse
	err := r.client.Call(ctx, method, p, &out)
	if err == nil && out.Thread.ID != "" {
		r.mu.Lock()
		r.sessions[out.Thread.ID] = struct{}{}
		r.mu.Unlock()
	}
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

// GetSessionStatus 查询服务器当前状态，不加载历史轮次，不恢复会话或建立订阅。
// 不存在的会话及服务端错误通过 error 返回；空状态不推断为空闲状态。
func (r *Runtime) GetSessionStatus(ctx context.Context, id string) (base.SessionStatus, error) {
	var out ThreadResponse
	if err := r.client.Call(
		ctx,
		"thread/read",
		ThreadReadParams{
			ThreadID:     id,
			IncludeTurns: false,
		},
		&out,
	); err != nil {
		return base.SessionStatus{}, err
	}
	return sessionStatus(out.Thread.Status), nil
}

// ListTurns 查询会话的轮次分页，不加载整个会话历史。
func (r *Runtime) ListTurns(ctx context.Context, p base.ListTurnsParams) (base.TurnPage, error) {
	var out ThreadTurnsListResponse
	err := r.client.Call(
		ctx,
		"thread/turns/list",
		ThreadTurnsListParams{
			ThreadID:      p.SessionID,
			Cursor:        p.Cursor,
			Limit:         p.Limit,
			SortDirection: string(p.SortDirection),
			ItemsView:     string(p.ItemsView),
		},
		&out,
	)
	if err != nil {
		return base.TurnPage{}, err
	}
	page := base.TurnPage{
		Turns:           make([]base.Turn, len(out.Data)),
		NextCursor:      out.NextCursor,
		BackwardsCursor: out.BackwardsCursor,
	}
	for i, t := range out.Data {
		page.Turns[i] = turn(t)
	}
	return page, nil
}
