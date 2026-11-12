package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/runtime"
	"github.com/hjhsamuel/magent/internal/runtime/codex"
	"github.com/hjhsamuel/magent/internal/signal"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/hjhsamuel/magent/pkg/token"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) Listen(
	ctx *gin.Context,
	auth *utils.Auth,
	sessionId string,
) error {
	id, r, err := s.initRuntime(ctx, auth, sessionId)
	if err != nil {
		return err
	}
	defer s.manager.Remove(id)

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")

	ctx.SSEvent(string(signal.SSEInit), signal.SSEPayload{Data: id})

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	index := 0
	for {
		select {
		case e := <-r.Events():
			switch e.Type {
			case runtime.EventThinkingDelta:
				ctx.SSEvent(string(signal.SSEReasoning), signal.SSEPayload{Data: e.Text})
			case runtime.EventTextDelta:
				ctx.SSEvent(string(signal.SSEContent), signal.SSEPayload{Data: e.Text})
			case runtime.EventThinkingCompleted, runtime.EventTextCompleted:
				ctx.SSEvent(string(signal.SSEComplete), nil)
			case runtime.EventTurnCompleted:
				if index != 0 {
					ctx.Writer.Flush()
				}

				_ = s.store.TurnComplete(context.Background(), e.SessionID, e.TurnID, e.Text)
				ctx.SSEvent(string(signal.SSEEnd), nil)
			case runtime.EventApprovalRequested:
				// TODO
				ctx.SSEvent(string(signal.SSEApprove), signal.SSEPayload{
					Data: signal.ApproveEvent{
						//ID: e.
					},
				})
			case runtime.EventSessionClosed:
				if index != 0 {
					ctx.Writer.Flush()
				}
				return nil
			default:
				continue
			}

			index += 1
		case <-r.Done():
			return nil
		case <-ticker.C:
			if index != 0 {
				ctx.Writer.Flush()
				index = 0
			}
		}
	}
}

func (s *Service) initRuntime(
	ctx context.Context,
	auth *utils.Auth,
	sessionId string,
) (string, *runtime.ManagedRuntime, error) {
	var (
		node  string
		runId string
	)
	if sessionId != "" {
		sid, err := bson.ObjectIDFromHex(sessionId)
		if err != nil {
			return "", nil, fmt.Errorf("invalid session: %w", err)
		}
		obj, err := s.store.GetSession(ctx, bson.M{"_id": sid})
		if err != nil {
			return "", nil, fmt.Errorf("database: %w", err)
		}
		node, runId = obj.Node, obj.RunId
	} else {
		node = s.node
	}

	// 初始化连接
	r, err := codex.NewWebSocketRuntime(ctx, fmt.Sprintf("ws://%s", node), nil)
	if err != nil {
		return "", nil, err
	}
	_, err = r.Initialize(ctx, runtime.ClientInfo{
		Name:    "magent",
		Title:   "My Agent",
		Version: "1.0.0",
	})
	if err != nil {
		_ = r.Close()
		return "", nil, fmt.Errorf("init connection: %w", err)
	}

	if runId != "" {
		// 恢复
		status, err := r.GetSessionStatus(ctx, runId)
		if err != nil {
			_ = r.Close()
			return "", nil, fmt.Errorf("check session status: %w", err)
		}

		if status.Type == runtime.SessionSystemError {
			_ = r.Close()
			return "", nil, errors.New("system error")
		}

		_, err = r.ResumeSession(ctx, runtime.ResumeSessionParams{
			SessionID: runId,
			Options:   runtime.SessionOptions{},
		})
		if err != nil {
			_ = r.Close()
			return "", nil, fmt.Errorf("resume session: %w", err)
		}
	} else {
		// 新建
		cwd, err := s.getRuntimeWorkDir(auth)
		if err != nil {
			_ = r.Close()
			return "", nil, err
		}

		rs, err := r.StartSession(ctx, runtime.SessionOptions{
			WorkingDirectory: cwd,
		})
		if err != nil {
			_ = r.Close()
			return "", nil, fmt.Errorf("start session: %w", err)
		}

		curTime := time.Now()
		// TODO
		sid, err := s.store.CreateSession(context.Background(), &schema.Session{
			Name:      "",
			User:      fmt.Sprintf("%v", auth.Raw["id"]),
			Status:    schema.SessionStateCompleted,
			Node:      node,
			RunId:     rs.ID,
			CreatedAt: curTime,
			UpdatedAt: curTime,
		})
		if err != nil {
			_ = r.Close()
			return "", nil, fmt.Errorf("create session record: %w", err)
		}
		sessionId = sid.Hex()
	}

	// 注册
	mr, err := s.manager.Register(sessionId, r)
	if err != nil {
		_ = s.manager.Remove(sessionId)
	}

	return sessionId, mr, nil
}

func (s *Service) getRuntimeWorkDir(auth *utils.Auth) (string, error) {
	v, ok := auth.Raw["id"]
	if !ok {
		return "", errors.New("invalid token")
	}
	path := filepath.Join(s.workdir, fmt.Sprintf("%v", v))
	_, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		mcps, err := s.store.FindMCPs(context.Background(), bson.M{"status": schema.ToolStateOnline})
		if err != nil {
			return "", err
		}
		if len(mcps) == 0 {
			return path, nil
		}

		ut, err := token.GenerateJWTWithoutExp(auth.Raw, utils.GetJwtSalt())
		if err != nil {
			return "", err
		}
		bearer := fmt.Sprintf("Bearer %s", ut)

		codexConfig := &codex.Config{
			McpServers: make(map[string]codex.McpServerConfig),
		}
		for _, item := range mcps {
			var t string
			if item.Token == "" {
				t = bearer
			} else {
				t = fmt.Sprintf("Bearer %s", item.Token)
			}
			codexConfig.McpServers[item.Name] = codex.McpServerConfig{
				URL: item.Url,
				HTTPHeaders: map[string]string{
					"Authorization": t,
				},
			}
		}
		if err = codexConfig.WriteTo(path); err != nil {
			return "", err
		}
	}
	return path, nil
}
