package codex

import (
	"encoding/json"
	"strings"

	base "github.com/hjhsamuel/magent/internal/runtime"
)

// forwardEvents 过滤协议噪声，保留内容事件顺序。
// 不支持的服务端请求明确返回错误，避免静默丢弃导致轮次一直等待。
func (r *Runtime) forwardEvents() {
	defer close(r.events)

	for m := range r.client.events {
		if len(m.ID) > 0 {
			if err := r.client.Respond(
				m.ID,
				nil,
				&RPCError{Code: -32601, Message: "此适配器不支持交互式服务端请求"},
			); err != nil {
				return
			}
			continue
		}

		e, err := convertEvent(m)
		if err != nil {
			r.client.stop(err)
			return
		}

		if e.Type == "" {
			continue
		}

		select {
		case r.events <- e:
		case <-r.Done():
			return
		}
	}
}

// convertEvent 不转发工具执行、启动通知和未知扩展消息。
func convertEvent(m Message) (base.Event, error) {
	var e base.Event
	switch m.Method {
	case "thread/closed":
		var p ThreadClosedNotification
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}
		e.Type = base.EventSessionClosed
		e.SessionID = p.ThreadID
	case "turn/completed":
		var p TurnNotification
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}
		e.Type = base.EventTurnCompleted
		e.SessionID = p.ThreadID
		e.TurnID = p.Turn.ID
		e.Status = p.Turn.Status
		if p.Turn.Error != nil {
			e.Error = &base.TurnError{Message: p.Turn.Error.Message, Details: p.Turn.Error.AdditionalDetails}
		}
	case "item/agentMessage/delta", "item/reasoning/summaryTextDelta", "item/reasoning/textDelta":
		var p AgentMessageDelta
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}
		e.Type = base.EventTextDelta
		if m.Method != "item/agentMessage/delta" {
			e.Type = base.EventThinkingDelta
		}
		e.SessionID = p.ThreadID
		e.TurnID = p.TurnID
		e.ItemID = p.ItemID
		e.Text = p.Delta
	case "item/completed":
		var p ItemNotification
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}
		switch p.Item.Type {
		case "agentMessage":
			e.Type = base.EventTextCompleted
			e.Text = p.Item.Text
		case "reasoning":
			e.Type = base.EventThinkingCompleted
			// 优先使用公开摘要；无摘要时使用服务器返回的公开思考文本。
			parts := p.Item.Summary
			if len(parts) == 0 {
				parts = p.Item.ReasoningContent
			}
			e.Text = strings.Join(parts, "\n")
		default:
			return e, nil
		}
		e.SessionID = p.ThreadID
		e.TurnID = p.TurnID
		e.ItemID = p.Item.ID
	}
	return e, nil
}
