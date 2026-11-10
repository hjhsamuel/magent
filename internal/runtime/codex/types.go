package codex

import (
	"encoding/json"
	"fmt"
)

// Message 是双向 JSON-RPC 消息信封；协议省略 jsonrpc 字段。
// ID 保留原始 JSON，以同时支持服务器使用的字符串和整数标识。
type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

// RPCError 表示服务端返回的协议错误，Data 保留扩展诊断。
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("codex RPC %d: %s", e.Code, e.Message)
}

// ClientInfo 标识接入平台。
type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type InitializeParams struct {
	ClientInfo ClientInfo `json:"clientInfo"`
}

type InitializeResponse struct {
	UserAgent string `json:"userAgent"`
}

type EmptyResponse struct{}

// ThreadParams 指定新会话的模型、工作目录和执行约束。
type ThreadParams struct {
	Model          string `json:"model,omitempty"`
	Cwd            string `json:"cwd,omitempty"`
	ApprovalPolicy string `json:"approvalPolicy,omitempty"`
	Sandbox        string `json:"sandbox,omitempty"`
}

type ThreadIDParams struct {
	ThreadID string `json:"threadId"`
}

type ThreadReadParams struct {
	ThreadID     string `json:"threadId"`
	IncludeTurns bool   `json:"includeTurns"`
}

type ThreadResumeParams struct {
	ThreadID string `json:"threadId"`
	ThreadParams
}

type ThreadListParams struct {
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Archived bool   `json:"archived"`
}

type ThreadListResponse struct {
	Data       []Thread `json:"data"`
	NextCursor *string  `json:"nextCursor"`
}

type ThreadResponse struct {
	Thread Thread `json:"thread"`
}

// Thread 是服务器持久化的会话；历史由服务器维护。
type Thread struct {
	ID        string `json:"id"`
	Preview   string `json:"preview"`
	Cwd       string `json:"cwd"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	Turns     []Turn `json:"turns"`
}

// Input 支持文本、远程图片及本地图片输入。
type Input struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
	Path string `json:"path,omitempty"`
}

type TurnStartParams struct {
	ThreadID string  `json:"threadId"`
	Input    []Input `json:"input"`
	Model    string  `json:"model,omitempty"`
}

type TurnInterruptParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type TurnResponse struct {
	Turn Turn `json:"turn"`
}

type Turn struct {
	ID     string     `json:"id"`
	Status string     `json:"status"`
	Items  []Item     `json:"items"`
	Error  *TurnError `json:"error"`
}

type TurnError struct {
	Message           string          `json:"message"`
	CodexErrorInfo    json.RawMessage `json:"codexErrorInfo"`
	AdditionalDetails *string         `json:"additionalDetails"`
}

// Item 映射消息项公共字段；工具专属字段可从通知原始参数解析。
type Item struct {
	Summary          []string `json:"summary,omitempty"`
	ReasoningContent []string `json:"-"`
	ID               string   `json:"id"`
	Type             string   `json:"type"`
	Text             string   `json:"text,omitempty"`
	Content          []Input  `json:"content,omitempty"`
}

type TurnNotification struct {
	ThreadID string `json:"threadId"`
	Turn     Turn   `json:"turn"`
}

type ItemNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Item     Item   `json:"item"`
}

type AgentMessageDelta struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	ItemID   string `json:"itemId"`
	Delta    string `json:"delta"`
}

// UnmarshalJSON 区分 reasoning 的字符串数组与 userMessage 的输入数组，
// 防止不同消息项复用 content 字段时发生解码失败。
func (i *Item) UnmarshalJSON(data []byte) error {
	type plain Item
	var wire struct {
		*plain
		Content json.RawMessage `json:"content"`
	}
	*i = Item{}
	wire.plain = (*plain)(i)
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	if len(wire.Content) == 0 {
		return nil
	}

	if i.Type == "reasoning" {
		return json.Unmarshal(wire.Content, &i.ReasoningContent)
	}

	return json.Unmarshal(wire.Content, &i.Content)
}
