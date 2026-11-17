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
			s.parseRuntimeMessage(producer, e)
		case <-r.Done():
			return
		}
	}
}

func (s *Service) parseRuntimeMessage(producer *ringbuffer.Producer, e codex.Message) {
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
			return
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventReasoningDelta,
			Data: params.Delta,
		})
	case codex.MethodAgentMessageDelta:
		var params struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventMessageDelta,
			Data: params.Delta,
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
			return
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
				return
			}
			content = strings.Join(parts, "\n")
		} else {
			return
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventComplete,
			Data: content,
		})
	case codex.MethodError:
		var params struct {
			Error struct {
				Message           string `json:"message"`
				AdditionalDetails string `json:"additionalDetails"`
			} `json:"error"`
		}
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return
		}
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventError,
			Data: ringbuffer.ErrorEvent{
				Message:           params.Error.Message,
				AdditionalDetails: params.Error.AdditionalDetails,
			},
		})
	case codex.MethodCommandExecutionApproval:
		// TODO
	case codex.MethodFileChangeApproval:
		// TODO
	case codex.MethodPermissionsApproval:
		// TODO
	case codex.MethodMcpServerElicitation:
		// TODO
	default:
		return
	}
}
