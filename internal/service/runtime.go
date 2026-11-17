package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) Chat(
	auth *utils.Auth,
	sessionId string,
	content string,
) error {
	sid, err := bson.ObjectIDFromHex(sessionId)
	if err != nil {
		return err
	}
	obj, err := s.store.GetSession(context.Background(), bson.M{"_id": sid, "user": auth.ID})
	if err != nil {
		return err
	}
	if obj.Status == schema.SessionStateRunning {
		return errors.New("session is running")
	}
	if obj.RunId == "" {
		return errors.New("invalid session")
	}

	buffer, err := s.rb.CreateOrGet(obj.RunId)
	if err != nil {
		return err
	}
	producer, err := buffer.ConnectProducer()
	if err != nil {
		return err
	}

	r, err := s.cm.OpenRuntime(context.Background(), obj.RunId)
	if err != nil {
		return err
	}

	go s.turnStart(content, r, producer)
	return nil
}

func (s *Service) turnStart(
	content string,
	r *codex.Runtime,
	producer *ringbuffer.Producer,
) {
	defer producer.Close()
	defer r.Close()

	_, err := r.TurnStart(context.Background(), codex.TurnStartParams{
		ThreadID: r.ThreadID(),
		Input:    []codex.TextInput{{Type: "text", Text: content}},
	})
	if err != nil {
		return
	}

	for {
		select {
		case e := <-r.Events():
			if s.parseRuntimeMessage(producer, e) {
				// TODO
				// 退出循环，结束处理
			}
		case <-r.Done():
			return
		}
	}
}

func (s *Service) parseRuntimeMessage(producer *ringbuffer.Producer, e codex.Message) bool {
	switch e.Method {
	case codex.MethodItemCompleted:
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventPartEnd,
		})
	case codex.MethodReasoningDelta:
		var params struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventReasoningDelta,
			Data: ringbuffer.CommonEvent{Data: params.Delta},
		})
	case codex.MethodAgentMessageDelta:
		var params struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventMessageDelta,
			Data: ringbuffer.CommonEvent{Data: params.Delta},
		})
	case codex.MethodTurnCompleted:
		var params struct {
			Turn *struct {
				Items []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"items"`
			} `json:"turn"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return true
		}

		var content string
		if params.Error != nil {
			content = params.Error.Message
		} else if params.Turn != nil {
			parts := make([]string, 0)
			for _, item := range params.Turn.Items {
				if item.Type != "agentMessage" {
					continue
				}
				parts = append(parts, item.Text)
			}
			if len(parts) == 0 {
				return true
			}
			content = strings.Join(parts, "\n")
		} else {
			return true
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventComplete,
			Data: ringbuffer.CommonEvent{Data: content},
		})
		// 接收到 turn/completed 信号，表示轮次结束
		return true
	case codex.MethodError:
		var params struct {
			Error struct {
				Message           string `json:"message"`
				AdditionalDetails string `json:"additionalDetails"`
			} `json:"error"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventError,
			Data: ringbuffer.ErrorEvent{
				Message:           params.Error.Message,
				AdditionalDetails: params.Error.AdditionalDetails,
			},
		})
	case codex.MethodCommandExecutionApproval:
		var params codex.CommandExecutionApprovalParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}

		parts := make([]string, 0)
		if params.Reason != nil {
			parts = append(parts, *params.Reason)
		}
		if params.Command != nil {
			parts = append(parts, "`"+*params.Command+"`")
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.ApproveReq{
				ID:      e.ID,
				Content: strings.Join(parts, "\n"),
			},
		})
	case codex.MethodFileChangeApproval:
		var params codex.FileChangeApprovalParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}

		var content string
		if params.Reason != nil {
			content = *params.Reason
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.ApproveReq{
				ID:      e.ID,
				Content: content,
			},
		})
	case codex.MethodPermissionsApproval:
		var params codex.PermissionsRequestApprovalParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}

		var content string
		if params.Reason != nil {
			content = *params.Reason
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.PermissionReq{
				ID:         e.ID,
				Content:    content,
				Permission: params.Permissions,
			},
		})
	case codex.MethodMcpServerElicitation:
		var params codex.McpServerElicitationRequestParams
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return false
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventApprove,
			Data: ringbuffer.ElicitationReq{
				ID:     e.ID,
				Schema: params.RequestedSchema,
			},
		})
	default:
	}

	return false
}
