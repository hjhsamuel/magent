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
		// Drain notifications while closing so unsubscribe responses can arrive
		// even when the caller has stopped consuming Events.
		select {
		case <-r.closing:
			continue
		default:
		}
		if len(m.ID) > 0 && !isApprovalMethod(m.Method) {
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
		case <-r.closing:
		case <-r.Done():
			return
		}
	}
}

// convertEvent 不转发工具执行、启动通知和未知扩展消息。
func convertEvent(m Message) (base.Event, error) {
	var e base.Event
	switch m.Method {
	case "error":
		var p ErrorNotification
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}

		e.Type = base.EventError
		e.SessionID, e.TurnID = p.ThreadID, p.TurnID
		e.Error = &base.TurnError{
			Message: p.Error.Message,
			Details: p.Error.AdditionalDetails,
		}
		e.WillRetry = p.WillRetry
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval":
		// Notifications cannot be answered and must not create approval prompts.
		if len(m.ID) == 0 || string(m.ID) == "null" {
			return e, nil
		}

		var p struct {
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
			ItemID   string `json:"itemId"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}

		e.Type = base.EventApprovalRequested
		content, err := approvalContent(m.Method, m.Params)
		if err != nil {
			return e, err
		}
		e.Text = content
		e.SessionID, e.TurnID, e.ItemID = p.ThreadID, p.TurnID, p.ItemID
		e.Request = &base.ApprovalRequest{
			ID:     append(json.RawMessage(nil), m.ID...),
			Method: m.Method,
			Params: append(json.RawMessage(nil), m.Params...),
		}
	case "serverRequest/resolved":
		var p struct {
			ThreadID  string          `json:"threadId"`
			RequestID json.RawMessage `json:"requestId"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return e, err
		}
		e.Type, e.SessionID, e.RequestID = base.EventRequestResolved, p.ThreadID, p.RequestID
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

func isApprovalMethod(method string) bool {
	switch method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval":
		return true
	default:
		return false
	}
}
