package codex

import base "github.com/hjhsamuel/magent/internal/runtime"

func session(t Thread) base.Session {
	out := base.Session{
		Status:           sessionStatus(t.Status),
		ID:               t.ID,
		Preview:          t.Preview,
		WorkingDirectory: t.Cwd,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		Turns:            make([]base.Turn, len(t.Turns)),
	}
	for i, v := range t.Turns {
		out.Turns[i] = turn(v)
	}
	return out
}

func turn(t Turn) base.Turn {
	out := base.Turn{
		StartedAt: t.StartedAt, CompletedAt: t.CompletedAt, DurationMs: t.DurationMs,
		RootTurnID: t.RootTurnID, ItemsView: base.TurnItemsView(t.ItemsView),
		ID:     t.ID,
		Status: t.Status,
		Items:  make([]base.Item, len(t.Items)),
	}
	// 协议规定旧响应缺省 itemsView 时视为 full。
	if out.ItemsView == "" {
		out.ItemsView = base.TurnItemsFull
	}
	for i, v := range t.Items {
		out.Items[i] = item(v)
	}
	if t.Error != nil {
		out.Error = &base.TurnError{
			Message: t.Error.Message,
			Details: t.Error.AdditionalDetails,
		}
	}
	return out
}

func item(t Item) base.Item {
	out := base.Item{
		ID:      t.ID,
		Type:    t.Type,
		Text:    t.Text,
		Content: make([]base.Input, len(t.Content)),
	}
	for i, v := range t.Content {
		out.Content[i] = base.Input{
			Type: v.Type,
			Text: v.Text,
			URL:  v.URL,
			Path: v.Path,
		}
	}
	return out
}

// sessionStatus 复制标记，避免协议对象与公共返回值共享可变切片。
func sessionStatus(s ThreadStatus) base.SessionStatus {
	out := base.SessionStatus{Type: base.SessionState(s.Type)}
	if s.ActiveFlags != nil {
		out.ActiveFlags = make([]base.SessionActiveFlag, len(s.ActiveFlags))
		for i, flag := range s.ActiveFlags {
			out.ActiveFlags[i] = base.SessionActiveFlag(flag)
		}
	}
	return out
}
