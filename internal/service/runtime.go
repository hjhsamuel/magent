package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
		producer.Close()
		return err
	}

	go s.turnStart(sid, obj.Seq, r, producer)

	turnId, err := s.store.StartTurn(context.Background(), sid, content)
	if err != nil {
		_ = r.Close()
		producer.Close()
		return err
	}

	response, err := r.TurnStart(context.Background(), codex.TurnStartParams{
		ThreadID: r.ThreadID(),
		Input:    []codex.TextInput{{Type: "text", Text: content}},
	})
	if err != nil {
		_ = r.Close()
		producer.Close()
		_ = s.store.TurnComplete(
			context.Background(),
			obj.RunId,
			bson.M{"_id": turnId},
			&schema.Message{
				Role:      schema.MessageRoleModel,
				Content:   err.Error(),
				CreatedAt: time.Now(),
			},
		)
		return err
	} else {
		_ = s.store.UpdateTurn(
			context.Background(),
			bson.M{"_id": turnId},
			bson.M{"$set": bson.M{"turn_id": response.Turn.ID}},
		)
	}

	return nil
}

func (s *Service) turnStart(
	sessionId bson.ObjectID,
	turnIndex uint64,
	r *codex.Runtime,
	producer *ringbuffer.Producer,
) {
	defer producer.Close()
	defer r.Close()

	for {
		select {
		case e := <-r.Events():
			completed := s.parseRuntimeMessage(producer, e)
			if completed == nil {
				continue
			}

			_ = s.store.TurnComplete(
				context.Background(),
				r.ThreadID(),
				bson.M{"session": sessionId, "index": turnIndex},
				&schema.Message{
					Role:      schema.MessageRoleModel,
					Content:   completed.Summary,
					CreatedAt: time.Now(),
				},
			)
			_, _ = producer.Write(ringbuffer.Payload{
				Type: ringbuffer.SSEventComplete,
			})

			return
		case <-r.Done():
			return
		}
	}
}

// parseRuntimeMessage returns a completion even when its summary is empty.
// The caller persists it before publishing the complete event.
func (s *Service) parseRuntimeMessage(producer *ringbuffer.Producer, e codex.Message) *completedTurn {
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
			return nil
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
			return nil
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventMessageDelta,
			Data: ringbuffer.CommonEvent{Data: params.Delta},
		})
	case codex.MethodTurnCompleted:
		var params codex.TurnNotification
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return &completedTurn{
				Summary: err.Error(),
			}
		}
		return &completedTurn{
			TurnID:  params.Turn.ID,
			Summary: turnSummary(params.Turn),
		}
	case codex.MethodError:
		var params struct {
			Error struct {
				Message           string `json:"message"`
				AdditionalDetails string `json:"additionalDetails"`
			} `json:"error"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil
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
			return nil
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
			return nil
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
			return nil
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
			return nil
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

	return nil
}

func (s *Service) Approve(
	sessionId string,
	id json.RawMessage,
	accept bool,
	method string,
	result json.RawMessage,
) error {
	var decision codex.ApprovalDecision
	if accept {
		decision = codex.ApprovalAccept
	} else {
		decision = codex.ApprovalDecline
	}

	sid, err := bson.ObjectIDFromHex(sessionId)
	if err != nil {
		return err
	}
	obj, err := s.store.GetSession(context.Background(), bson.M{"_id": sid})
	if err != nil {
		return err
	}
	// 不在运行状态，说明轮次已结束
	if obj.Status != schema.SessionStateRunning {
		return errors.New("id not found")
	}

	r := s.cm.GetRuntime(obj.RunId)
	if r == nil {
		return errors.New("runtime not found")
	}
	switch codex.Method(method) {
	case codex.MethodCommandExecutionApproval, codex.MethodFileChangeApproval:
		return r.Approve(context.Background(), id, decision)
	case codex.MethodPermissionsApproval:
		var permission codex.GrantedPermissionProfile
		if decision == codex.ApprovalAccept {
			if err = json.Unmarshal(result, &permission); err != nil {
				return err
			}
		}
		return r.ApprovePermissions(context.Background(), id, decision, permission)
	case codex.MethodMcpServerElicitation:
		return r.RespondElicitation(context.Background(), id, decision, result)
	default:
		return errors.New("unknown method")
	}
}
