package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/prompts"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) Listen(
	c *gin.Context,
	auth *utils.Auth,
	sessionId string,
	seq uint64,
) error {
	ctx := c.Request.Context()

	cs, err := s.initUserWorkspace(ctx, auth)
	if err != nil {
		return err
	}

	var (
		id    string
		runId string
	)
	if sessionId == "" {
		r, err := cs.OpenRuntime(context.Background(), "")
		if err != nil {
			return err
		}
		runId = r.ThreadID()

		sid, err := s.store.CreateSession(ctx, &schema.Session{
			Name:      "New Chat",
			User:      auth.ID,
			Status:    schema.SessionStateInit,
			RunId:     runId,
			CreatedAt: time.Now(),
		})
		if err != nil {
			_ = r.Close()
			return err
		}
		id = sid.Hex()
	} else {
		sid, err := bson.ObjectIDFromHex(sessionId)
		if err != nil {
			return err
		}
		obj, err := s.store.GetSession(ctx, bson.M{"_id": sid, "user": auth.ID})
		if err != nil {
			return err
		}
		runId, id = obj.RunId, sessionId
	}

	buffer, err := s.rb.CreateOrGet(runId)
	if err != nil {
		return err
	}
	consumer, err := buffer.ConnectConsumer(seq)
	if err != nil {
		// 仅当为新会话时关闭 ringbuffer
		if sessionId == "" {
			buffer.Close()
		}
		return err
	}
	defer consumer.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 补偿：可能因断连导致的审批请求未送达
	if r := cs.GetRuntime(runId); r != nil {
		approveReqs := r.ApprovalRequests()
		if len(approveReqs) != 0 {
			for _, req := range approveReqs {
				event, _, err := parseRuntimeMessage(req)
				if err != nil {
					continue
				}
				_ = s.pushSSEvent(c, *event)
			}
		}
	}

	if err = s.pushSSEvent(c, ringbuffer.Payload{
		Type: ringbuffer.SSEventInit,
		Data: ringbuffer.CommonEvent{Data: id},
	}); err != nil {
		return err
	}

	events, err := consumer.ReadAll()
	if err != nil {
		return err
	}
	if err = s.pushSSEvent(c, events...); err != nil {
		return err
	}

	for {
		item, err := consumer.Next(ctx)
		if err != nil {
			if errors.Is(err, ctx.Err()) || errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err = s.pushSSEvent(c, item); err != nil {
			return err
		}
	}
}

func (s *Service) pushSSEvent(ctx *gin.Context, events ...ringbuffer.Payload) error {
	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		select {
		case <-ctx.Request.Context().Done():
			return ctx.Err()
		default:
			ctx.Render(-1, sse.Event{
				Event: string(event.Type),
				Id:    strconv.FormatUint(event.ID, 10),
				Data:  event.Data,
			})
		}
	}
	ctx.Writer.Flush()

	return nil
}

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

	cs := s.cm.Get(auth.ID)
	if cs == nil {
		producer.Close()
		return err
	}
	r, err := cs.OpenRuntime(context.Background(), obj.RunId)
	if err != nil {
		producer.Close()
		return err
	}

	turnId, index, err := s.store.StartTurn(context.Background(), sid, content)
	if err != nil {
		_ = r.Close()
		producer.Close()
		return err
	}

	go s.turnStart(sid, index, r, producer, content)

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
	}

	_ = s.store.UpdateTurn(
		context.Background(),
		bson.M{"_id": turnId},
		bson.M{"$set": bson.M{"turn_id": response.Turn.ID}},
	)

	return nil
}

func (s *Service) turnStart(
	sessionId bson.ObjectID,
	turnIndex uint64,
	r *codex.Runtime,
	producer *ringbuffer.Producer,
	content string,
) {
	defer producer.Close()
	defer r.Close()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go s.solveRuntimeEvents(wg, sessionId, turnIndex, r, producer)
	if turnIndex == 0 && content != "" {
		wg.Add(1)
		go s.generateSessionTitle(wg, sessionId, content, producer)
	}

	wg.Wait()
}

func (s *Service) solveRuntimeEvents(
	wg *sync.WaitGroup,
	sessionId bson.ObjectID,
	turnIndex uint64,
	r *codex.Runtime,
	producer *ringbuffer.Producer,
) {
	defer wg.Done()

	for {
		select {
		case e := <-r.Events():
			payload, completed, err := parseRuntimeMessage(e)
			if err != nil {
				if e.Method != codex.MethodTurnCompleted {
					continue
				}
				completed = &completedTurn{Error: err.Error()}
			}
			if payload != nil {
				_, _ = producer.Write(*payload)
			}
			if completed == nil {
				continue
			}

			var message *schema.Message
			if completed.Summary != "" {
				message = &schema.Message{
					Role:      schema.MessageRoleModel,
					Content:   completed.Summary,
					CreatedAt: time.Now(),
				}
			} else {
				message = &schema.Message{
					Role:      schema.MessageRoleModelError,
					Content:   completed.Error,
					CreatedAt: time.Now(),
				}
			}
			_ = s.store.TurnComplete(
				context.Background(),
				r.ThreadID(),
				bson.M{"session": sessionId, "index": turnIndex},
				message,
			)
			_, _ = producer.Write(ringbuffer.Payload{
				Type: ringbuffer.SSEventComplete,
				Data: message.Content,
			})

			return
		case <-r.Done():
			return
		}
	}
}

func (s *Service) generateSessionTitle(
	wg *sync.WaitGroup,
	sid bson.ObjectID,
	content string,
	producer *ringbuffer.Producer,
) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	title, err := s.topic.Complete(ctx, "", prompts.TopicPrompt, content)
	if err == nil {
		err = s.store.UpdateSession(
			context.Background(),
			bson.M{"_id": sid},
			bson.M{"$set": bson.M{"name": title}},
		)
		_, _ = producer.Write(ringbuffer.Payload{
			Type: ringbuffer.SSEventTitle,
			Data: ringbuffer.CommonEvent{Data: title},
		})
	}
	if err != nil {
		logrus.WithError(err).WithField("session", sid.Hex()).Warn("could not generate session title")
	}
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

	server := s.cm.Get(obj.User)
	if server == nil {
		return errors.New("runtime not found")
	}
	r := server.GetRuntime(obj.RunId)
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
