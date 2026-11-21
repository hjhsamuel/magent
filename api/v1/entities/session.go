package entities

type StreamReq struct {
	ID  string `form:"id"`  // 会话 id
	Seq uint64 `form:"seq"` // sse chunk id
}
