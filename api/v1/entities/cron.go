package entities

type GetCronListReq struct {
	Offset string `form:"offset"` // 检索游标
	Limit  int64  `form:"limit"`  // 返回数量
	Status *int   `form:"status"` // 状态。0：在线；1：离线
}

type GetCronListRsp struct {
	Offset string      `json:"offset"` // 下次检索游标
	Data   []*CronInfo `json:"data"`
}

type CronInfo struct {
	ID        string `json:"id"`         // 任务 id
	Name      string `json:"name"`       // 名称
	Schedule  string `json:"schedule"`   // 执行周期
	Content   string `json:"content"`    // 输入信息
	Status    int    `json:"status"`     // 状态。0：在线；1：离线
	Creator   int    `json:"creator"`    // 创建者
	CreatedAt int64  `json:"created_at"` // 创建时间
}

type DeleteCronJobReq struct {
	ID string `uri:"id"` // 任务 id
}

type AddCronJobReq struct {
	Name    string `json:"name"`    // 名称
	Spec    string `json:"spec"`    // 执行周期
	Content string `json:"content"` // 消息
}

type UpdateCronJobReq struct {
	ID      string  `uri:"id"`       // 任务 id
	Name    *string `json:"name"`    // 名称
	Spec    *string `json:"spec"`    // 执行周期
	Content *string `json:"content"` // 输入信息
	Status  *int    `json:"status"`  // 状态。0：在线；1：离线
}
