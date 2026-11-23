package ringbuffer

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	Capacity        = 1024
	MaxIdleDuration = 10 * time.Second
)

var (
	ErrClosed            = errors.New("sse: closed")
	ErrProducerConnected = errors.New("sse: producer already connected")
	ErrConsumerConnected = errors.New("sse: consumer already connected")
	ErrExists            = errors.New("sse: stream already exists")
)

type Payload struct {
	ID   uint64
	Type SSEvent
	Data any // 除了 approve，其余都为 CommonEvent 或空
}

type SSEvent string

const (
	SSEventInit           SSEvent = "init"            // 完成初始化
	SSEventTitle          SSEvent = "title"           // 标题
	SSEventPartEnd        SSEvent = "part_end"        // 中间段落结束
	SSEventReasoningDelta SSEvent = "reasoning_delta" // 思考片段
	SSEventReasoning      SSEvent = "reasoning"       // 思考内容
	SSEventMessageDelta   SSEvent = "message_delta"   // 正文片段
	SSEventMessage        SSEvent = "message"         // 正文内容
	SSEventComplete       SSEvent = "complete"        // 对话结束
	SSEventApprove        SSEvent = "approve"         // 审批请求
	SSEventError          SSEvent = "error"           // 错误信息
)

// CommonEvent 将普通的数据进行结构体封装，避免浏览器将 sse 数据的第一个空格移除
type CommonEvent struct {
	Data string `json:"data"`
}

type ApproveReq struct {
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Content string          `json:"content"`
}

type PermissionReq struct {
	ID         json.RawMessage `json:"id"`
	Method     string          `json:"method"`
	Content    string          `json:"content"`
	Permission any             `json:"permission"`
}

type ElicitationReq struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Schema json.RawMessage `json:"schema"`
}

type ErrorEvent struct {
	Message           string `json:"message"`
	AdditionalDetails string `json:"additional_details"`
}
