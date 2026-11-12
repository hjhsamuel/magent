package signal

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
	ID      string `json:"id"`
	Content string `json:"content"`
}
