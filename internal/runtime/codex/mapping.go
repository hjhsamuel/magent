package codex

import base "github.com/hjhsamuel/magent/internal/runtime"

func session(t Thread) base.Session {
	out := base.Session{
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
		ID:     t.ID,
		Status: t.Status,
		Items:  make([]base.Item, len(t.Items)),
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
