package entities

type GetToolListReq struct {
	Kind   int    `form:"kind"`   // 工具类型。0：mcp；1：skill
	Offset string `form:"offset"` // 检索游标
	Limit  int64  `form:"limit"`  // 检索数量
}

type GetToolListRsp struct {
	Offset string      `json:"offset"` // 继续检索游标
	Data   []*ToolBase `json:"data"`
}

type ToolBase struct {
	ID          string `json:"id"`          // 工具 id
	Name        string `json:"name"`        // 名称
	Description string `json:"description"` // 描述
	Status      int    `json:"status"`      // 状态。0：已上线；1：上线中；2：安装中；3：离线
	Error       string `json:"error"`       // 上线失败原因
	Creator     int    `json:"creator"`     // 创建者
	CreatedAt   int64  `json:"created_at"`  // 创建时间
	UpdatedAt   int64  `json:"updated_at"`  // 更新时间
}

type AddToolReq struct {
	Name        string `json:"name"`        // 名称
	Description string `json:"description"` // 描述
	Url         string `json:"url"`         // 远程地址。mcp：mcp server 地址；skill：压缩包下载地址
	Kind        int    `json:"kind"`        // 类型。0：mcp；1：skill
	Token       string `json:"token"`       // Bearer token。未传递将使用服务内部 token
}
