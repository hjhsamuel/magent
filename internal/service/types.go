package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
)

const (
	WorkspaceDir = "workspace"

	UserWorkDirName = "user"
	SkillDirName    = "skills"
	TempDirName     = "temp"
)

type completedTurn struct {
	TurnID  string
	Summary string
	Error   string
}

func parseRuntimeMessage(e codex.Message) (*ringbuffer.Payload, *completedTurn, error) {
	switch e.Method {
	case codex.MethodItemCompleted:
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventPartEnd,
		}, nil, nil
	case codex.MethodReasoningDelta, codex.MethodReasoningSummaryDelta:
		var params struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventReasoningDelta,
			Data: ringbuffer.CommonEvent{Data: params.Delta},
		}, nil, nil
	case codex.MethodAgentMessageDelta:
		var params struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventMessageDelta,
			Data: ringbuffer.CommonEvent{Data: params.Delta},
		}, nil, nil
	case codex.MethodTurnCompleted:
		var params codex.TurnNotification
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}
		if params.Turn.ID == "" {
			return nil, nil, errors.New("completion missing turn ID")
		}
		if params.Turn.Error != nil {
			return nil, &completedTurn{
				TurnID: params.Turn.ID,
				Error:  params.Turn.Error.Message,
			}, nil
		} else {
			var parts []string
			for _, raw := range params.Turn.Items {
				var item codex.SummaryItem
				if json.Unmarshal(raw, &item) == nil && item.Type == codex.AgentMessage {
					parts = append(parts, item.Text)
				}
			}
			return nil, &completedTurn{
				TurnID:  params.Turn.ID,
				Summary: strings.Join(parts, "\n"),
			}, nil
		}
	case codex.MethodError:
		var params struct {
			Error struct {
				Message           string `json:"message"`
				AdditionalDetails string `json:"additionalDetails"`
			} `json:"error"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventError,
			Data: ringbuffer.ErrorEvent{
				Message:           params.Error.Message,
				AdditionalDetails: params.Error.AdditionalDetails,
			},
		}, nil, nil
	case codex.MethodCommandExecutionApproval:
		var params codex.CommandExecutionApprovalParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}

		parts := make([]string, 0)
		if params.Reason != nil {
			parts = append(parts, *params.Reason)
		}
		if params.Command != nil {
			parts = append(parts, "`"+*params.Command+"`")
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.ApproveReq{
				ID:      e.ID,
				Method:  string(codex.MethodCommandExecutionApproval),
				Content: strings.Join(parts, "\n"),
			},
		}, nil, nil
	case codex.MethodFileChangeApproval:
		var params codex.FileChangeApprovalParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}

		var content string
		if params.Reason != nil {
			content = *params.Reason
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.ApproveReq{
				ID:      e.ID,
				Method:  string(codex.MethodFileChangeApproval),
				Content: content,
			},
		}, nil, nil
	case codex.MethodPermissionsApproval:
		var params codex.PermissionsRequestApprovalParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}

		var content string
		if params.Reason != nil {
			content = *params.Reason
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.PermissionReq{
				ID:         e.ID,
				Method:     string(codex.MethodPermissionsApproval),
				Content:    content,
				Permission: params.Permissions,
			},
		}, nil, nil
	case codex.MethodMcpServerElicitation:
		var params codex.McpServerElicitationRequestParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, nil, err
		}
		return &ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.ElicitationReq{
				ID:      e.ID,
				Method:  string(codex.MethodMcpServerElicitation),
				Content: params.Message,
				Schema:  params.RequestedSchema,
			},
		}, nil, nil
	default:
		return nil, nil, nil
	}
}
