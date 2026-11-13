package signal

import "encoding/json"

type SSEChunkType string

const (
	SSEInit      SSEChunkType = "init"
	SSETitle     SSEChunkType = "title"
	SSEContent   SSEChunkType = "content"   // content delta
	SSEReasoning SSEChunkType = "reasoning" // reasoning delta
	SSEComplete  SSEChunkType = "complete"  // item complete
	SSEApprove   SSEChunkType = "approve"
	SSEError     SSEChunkType = "error"
	SSEEnd       SSEChunkType = "end" // turn complete
)

type SSEPayload struct {
	Data any `json:"data"`
}

type ApproveEvent struct {
	ID      string            `json:"id"`
	Content string            `json:"content"` // Markdown with escaped dynamic values.
	Raw     *RawApproveParams `json:"raw"`
}

type RawApproveParams struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
