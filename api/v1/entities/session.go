package entities

import "encoding/json"

type StreamReq struct {
	ID  string `form:"id"`  // 会话 id
	Seq uint64 `form:"seq"` // sse chunk id
}

type ChatReq struct {
	ID      string `uri:"id"`       // 会话 id
	Content string `json:"content"` // 消息
}

type ApprovalReq struct {
	ID     string       `uri:"id"`      // 会话 id
	Accept bool         `json:"accept"` // 审批结果
	Info   ApprovalInfo `json:"info"`   // 审批内容
}

type ApprovalInfo struct {
	ID     json.RawMessage `json:"id"`     // 审批 id
	Method string          `json:"method"` // 具体审批类型
	Params json.RawMessage `json:"params"` // 审批响应。权限：回传；mcp：JsonSchema 响应结果
}

type GetSessionListReq struct {
	Offset string `form:"offset"` // 检索游标
	Limit  int64  `form:"limit"`  // 检索数量
}

type GetSessionListRsp struct {
	Offset string         `json:"offset"` // 继续检索游标
	Data   []*SessionBase `json:"data"`
}

type SessionBase struct {
	ID        string `json:"id"`         // 会话 id
	Name      string `json:"name"`       // 标题
	Status    int    `json:"status"`     // 状态。0：新建；1：运行中；2：已完成
	CreatedAt int64  `json:"created_at"` // 创建时间
	UpdatedAt int64  `json:"updated_at"` // 更新时间
}

type GetTurnsReq struct {
	ID     string `uri:"id"`      // 会话 id
	Offset string `form:"offset"` // 检索游标
	Limit  int64  `form:"limit"`  // 检索数量
}

type GetTurnsRsp struct {
	Offset string      `json:"offset"` // 继续检索游标
	Data   []*TurnBase `json:"data"`
}

type TurnBase struct {
	ID       string     `json:"id"` // 轮次 id
	Messages []*Message `json:"messages"`
}

type Message struct {
	Role      string `json:"role"`       // 角色
	Content   string `json:"content"`    // 消息
	CreatedAt int64  `json:"created_at"` // 创建时间
}
