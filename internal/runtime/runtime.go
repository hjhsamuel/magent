package runtime

import (
	"context"
	"encoding/json"
)

// Runtime 是平台统一入口。实例拥有连接，调用方须持续读取 Events 并在结束时 Close。
// 同一会话应等待当前轮次完成后再提交下一轮；取消请求等待不等于中断推理。
type Runtime interface {
	Initialize(context.Context, ClientInfo) (InitializeResponse, error)
	StartSession(context.Context, SessionOptions) (Session, error)
	ResumeSession(context.Context, ResumeSessionParams) (Session, error)
	ReadSession(context.Context, ReadSessionParams) (Session, error)
	ListSessions(context.Context, ListSessionsParams) (SessionPage, error)
	ArchiveSession(context.Context, string) error
	UnarchiveSession(context.Context, string) (Session, error)
	StartTurn(context.Context, StartTurnParams) (Turn, error)
	InterruptTurn(context.Context, InterruptTurnParams) error
	Events() <-chan Event
	Respond(json.RawMessage, any, *ResponseError) error
	Done() <-chan struct{}
	Err() error
	Close() error
}

// ClientInfo 标识调用平台。
type ClientInfo struct {
	Name    string
	Title   string
	Version string
}

type InitializeResponse struct {
	UserAgent string
}

// SessionOptions 表示会话配置；具体策略值由适配器支持情况决定。
type SessionOptions struct {
	Model            string
	WorkingDirectory string
	ApprovalPolicy   string
	Sandbox          string
}

type ResumeSessionParams struct {
	SessionID string
	Options   SessionOptions
}

type ReadSessionParams struct {
	SessionID    string
	IncludeTurns bool
}

type ListSessionsParams struct {
	Cursor   string
	Limit    int
	Archived bool
}

type SessionPage struct {
	Sessions   []Session
	NextCursor *string
}

// Session 的时间字段为 Unix 秒，历史轮次按服务端返回顺序排列。
type Session struct {
	ID, Preview, WorkingDirectory string
	CreatedAt, UpdatedAt          int64
	Turns                         []Turn
}

type Input struct {
	Type string
	Text string
	URL  string
	Path string
}

type StartTurnParams struct {
	SessionID string
	Input     []Input
	Model     string
}

type InterruptTurnParams struct {
	SessionID string
	TurnID    string
}

type Turn struct {
	ID, Status string
	Items      []Item
	Error      *TurnError
}

type TurnError struct {
	Message string
	Details *string
}

type Item struct {
	ID, Type, Text string
	Content        []Input
}

// EventType 仅暴露用户可消费的内容和轮次结束信号。
type EventType string

const (
	EventTurnCompleted     EventType = "turn.completed"
	EventThinkingDelta     EventType = "thinking.delta"
	EventTextDelta         EventType = "text.delta"
	EventThinkingCompleted EventType = "thinking.completed"
	EventTextCompleted     EventType = "text.completed"
)

// Event 的 Text 在增量事件中是 chunk，在完成事件中是单个 Item 的全文。
// 同一轮可包含多个思考或正文 Item，使用 ItemID 区分；全文不应再次追加到 chunk。
// 思考内容仅包含服务端公开提供的内容。Status 和 Error 仅用于轮次结束。
// 连接异常由 Done 和 Err 表达，不伪造成功结束信号。
type Event struct {
	Type                      EventType
	SessionID, TurnID, ItemID string
	Text                      string
	Status                    string
	Error                     *TurnError
}

// ResponseError 用于回复无法处理的服务端请求。
type ResponseError struct {
	Code    int
	Message string
	Data    json.RawMessage
}
