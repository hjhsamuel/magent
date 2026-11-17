package service

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) Listen(
	ctx *gin.Context,
	auth *utils.Auth,
	sessionId string,
	seq uint64,
) error {
	var (
		id    string
		runId string
		err   error
	)
	if sessionId == "" {
		r, err := s.cm.OpenRuntime(context.Background(), "")
		if err != nil {
			return err
		}
		runId = r.ThreadID()

		sid, err := s.store.CreateSession(ctx, &schema.Session{
			Name:      "New Chat",
			User:      auth.ID,
			Status:    schema.SessionStateInit,
			Node:      s.node,
			RunId:     runId,
			CreatedAt: time.Now(),
		})
		if err != nil {
			_ = s.cm.RemoveRuntime(runId)
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

	buffer, exists := s.rb.Get(runId)
	if !exists {
		buffer, err = s.rb.Create(runId)
		if err != nil {
			return err
		}
	}
	consumer, err := buffer.ConnectConsumer(seq)
	if err != nil {
		return err
	}
	defer consumer.Close()

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")

	ctx.Render(-1, sse.Event{
		Event: string(ringbuffer.SSEventInit),
		Data:  id,
	})
	ctx.Writer.Flush()

	// 先一次性读取缓存 chunk
	chunks, err := consumer.ReadAll()
	if err != nil {
		return err
	}
	for _, item := range chunks {
		ctx.Render(-1, sse.Event{
			Event: string(item.Type),
			Id:    strconv.FormatUint(item.ID, 10),
			Data:  item.Data,
		})
	}
	ctx.Writer.Flush()

	for {
		item, err := consumer.Next(ctx)
		if err != nil {
			return err
		}
		ctx.Render(-1, sse.Event{
			Event: string(item.Type),
			Id:    strconv.FormatUint(item.ID, 10),
			Data:  item.Data,
		})
		ctx.Writer.Flush()
	}
}
