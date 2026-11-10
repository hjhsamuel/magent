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
	GetSessionStatus(context.Context, string) (SessionStatus, error)
	ListSessions(context.Context, ListSessionsParams) (SessionPage, error)
	ArchiveSession(context.Context, string) error
	UnsubscribeSession(context.Context, string) (UnsubscribeStatus, error)
	UnarchiveSession(context.Context, string) (Session, error)
	ListTurns(context.Context, ListTurnsParams) (TurnPage, error)
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
	Status                        SessionStatus
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
	// 时间戳单位为秒；耗时单位为毫秒，nil 表示服务器未提供。
	StartedAt, CompletedAt, DurationMs *int64
	RootTurnID                         *string
	ItemsView                          TurnItemsView
	ID, Status                         string
	Items                              []Item
	Error                              *TurnError
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
	// EventSessionClosed 表示当前连接不再接收该会话通知，不代表会话被删除。
	EventSessionClosed     EventType = "session.closed"
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

// UnsubscribeStatus 表示取消订阅请求的处理结果；后两种状态表示无需重复退订。
type UnsubscribeStatus string

const (
	Unsubscribed  UnsubscribeStatus = "unsubscribed"
	NotSubscribed UnsubscribeStatus = "notSubscribed"
	NotLoaded     UnsubscribeStatus = "notLoaded"
)

// SessionStatus 是服务器返回的会话运行状态快照，不代表持久化历史是否存在。
// Type 为空表示服务器未提供状态，不能当作 idle。
type SessionStatus struct {
	Type        SessionState
	ActiveFlags []SessionActiveFlag
}

// SessionState 与轮次状态独立；notLoaded 不等于会话不存在。
type SessionState string

const (
	SessionNotLoaded   SessionState = "notLoaded"
	SessionIdle        SessionState = "idle"
	SessionActive      SessionState = "active"
	SessionSystemError SessionState = "systemError"
)

// SessionActiveFlag 说明运行中的会话正在等待什么；空列表表示没有等待标记。
type SessionActiveFlag string

const (
	SessionWaitingOnApproval  SessionActiveFlag = "waitingOnApproval"
	SessionWaitingOnUserInput SessionActiveFlag = "waitingOnUserInput"
)

// ListTurnsParams 查询指定会话的轮次；游标为不透明值，须原样传回。
// 空排序默认倒序，空 ItemsView 默认摘要；Limit 为 nil 时使用服务端默认页大小。
type ListTurnsParams struct {
	SessionID     string
	Cursor        string
	Limit         *uint32
	SortDirection SortDirection
	ItemsView     TurnItemsView
}

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

// TurnItemsView 区分未加载、摘要与完整历史，避免将空 Items 误认为没有消息。
type TurnItemsView string

const (
	TurnItemsNotLoaded TurnItemsView = "notLoaded"
	TurnItemsSummary   TurnItemsView = "summary"
	TurnItemsFull      TurnItemsView = "full"
)

// TurnPage 保留服务器顺序，不在客户端重新排序。
// BackwardsCursor 用于反转 SortDirection 后查询，会再次包含锚点轮次以获取更新。
type TurnPage struct {
	Turns           []Turn
	NextCursor      *string
	BackwardsCursor *string
}
