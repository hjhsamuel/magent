package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/prompts"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/hjhsamuel/magent/pkg/cryptox"
	"github.com/hjhsamuel/magent/pkg/fx"
	"github.com/hjhsamuel/magent/pkg/token"
	"github.com/pelletier/go-toml/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Service) Listen(
	c *gin.Context,
	auth *utils.Auth,
	sessionId string,
	seq uint64,
) error {
	ctx := c.Request.Context()

	cs, err := s.initUserWorkspace(auth)
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

	go s.turnStart(sid, index, r, producer)
	if index == 0 {
		go s.generateSessionTitle(sid, content, producer)
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
) {
	defer producer.Close()
	defer r.Close()

	for {
		select {
		case e := <-r.Events():
			payload, completed, err := parseRuntimeMessage(e)
			if err != nil {
				if e.Method != codex.MethodTurnCompleted {
					continue
				}
				completed = &completedTurn{Summary: err.Error()}
			}
			if payload != nil {
				_, _ = producer.Write(*payload)
			}
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

func (s *Service) generateSessionTitle(
	sid bson.ObjectID,
	content string,
	producer *ringbuffer.Producer,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	title, err := s.topic.Complete(ctx, "", prompts.TopicPrompt, content)
	if err == nil {
		err = s.store.UpdateSession(
			ctx,
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

func (s *Service) initUserWorkspace(auth *utils.Auth) (*codex.UserRuntime, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检测 pod 是否存在
	podName := fmt.Sprintf("codex-%d", auth.ID)
	namespace := s.k8s.Namespace()
	_, err := s.k8s.GetPodStatus(ctx, namespace, podName)
	missing := apierrors.IsNotFound(err)
	if err != nil && !missing {
		return nil, err
	}

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+s.wsToken)
	ur, err := s.cm.CreateOrGet(auth.ID, headers)
	if err != nil {
		return nil, err
	}

	if missing {
		userToken, err := token.GenerateJWTWithoutExp(auth.Raw, utils.GetJwtSalt())
		if err != nil {
			return nil, err
		}

		// 创建用户工作目录
		if err = s.createUserWorkspace(auth); err != nil {
			return nil, err
		}

		// 创建pod
		serviceAccount := false
		_, err = s.k8s.CreatePod(
			ctx,
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels:    map[string]string{"agent": "codex"},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &serviceAccount,
					Containers: []corev1.Container{{
						Name:  podName,
						Image: "codex:latest",
						Command: []string{
							"codex",
							"app-server",
							"--listen",
							"ws://0.0.0.0:8000",
							"--ws-auth",
							"capability-token",
							"--ws-token-sha256",
							"$TOKEN_SHA256",
						},
						Env: []corev1.EnvVar{
							{Name: "CODEX_HOME", Value: "/app/.codex"},
							{Name: "TOKEN_SHA256", Value: cryptox.TokenSha256(s.wsToken)},
							{Name: "MAGENT_USER_TOKEN", Value: userToken},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "workspace",
								MountPath: "/app/.codex",
								SubPath:   fmt.Sprintf("%s/%d", UserWorkDirName, auth.ID),
							},
							{
								Name:      "workspace",
								MountPath: "/root/.agents/skills",
								SubPath:   SkillDirName,
								ReadOnly:  true,
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "agent-workspace",
								},
							},
						},
					},
				},
			},
		)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	return ur, nil
}

func (s *Service) createUserWorkspace(auth *utils.Auth) error {
	path, exists := s.isUserWorkspaceExists(auth)
	if exists {
		return nil
	}

	_ = os.MkdirAll(path, os.ModePerm)

	if err := fx.CopyFile(
		filepath.Join(WorkspaceDir, "models.json"),
		filepath.Join(path, "models.json"),
	); err != nil {
		return err
	}

	if err := fx.CopyFile(
		filepath.Join(WorkspaceDir, "config.toml"),
		filepath.Join(path, "config.toml"),
	); err != nil {
		return err
	}

	return nil
}

func (s *Service) updateAllUserWorkspace(hasMcp, hasSkill bool) error {
	// 更新文件
	var (
		rootPath = filepath.Join(WorkspaceDir, UserWorkDirName)
		cfgPath  = filepath.Join(WorkspaceDir, "config.toml")
	)
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return err
	}
	for _, item := range entries {
		_ = fx.CopyFile(
			cfgPath,
			filepath.Join(rootPath, item.Name()),
		)
	}
	// 获取 pod
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pods, err := s.k8s.GetPodList(ctx, s.k8s.Namespace(), "agent:codex")
	if err != nil {
		return err
	}
	// 发送信号
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+s.wsToken)
	for _, item := range pods {
		strId := strings.TrimPrefix(item.Name, "codex-")
		userId, err := strconv.Atoi(strId)
		if err != nil {
			continue
		}
		ur, err := s.cm.CreateOrGet(userId, headers)
		if err != nil {
			continue
		}
		_ = ur.ReloadSkillsAndMCPServers(context.Background(), hasMcp, hasSkill)
	}

	return nil
}

func (s *Service) updateWorkspaceConfig(tools ...*schema.Tool) (bool, bool, error) {
	if len(tools) == 0 {
		return false, false, nil
	}

	var (
		addMcps, delMcps       = make([]*schema.Tool, 0), make([]*schema.Tool, 0)
		updateMcp, updateSkill = false, false
	)
	for _, item := range tools {
		if item.Status != schema.ToolStatusReady && item.Status != schema.ToolStateUninstall {
			continue
		}

		switch item.Kind {
		case schema.ToolMcp:
			updateMcp = true
			// mcp 需要更新用户配置
			if item.Status == schema.ToolStatusReady {
				addMcps = append(addMcps, item)
			} else if item.Status == schema.ToolStateUninstall {
				delMcps = append(delMcps, item)
			}
		case schema.ToolSkill:
			// skill 目录已经更新，只需要发送信号即可
			updateSkill = true
		}
	}

	// 更新 config.toml
	if updateMcp {
		cfg, err := s.getWorkspaceConfig(addMcps, delMcps)
		if err != nil {
			return false, false, err
		}
		content, err := toml.Marshal(cfg)
		if err != nil {
			return false, false, err
		}
		err = os.WriteFile(filepath.Join(WorkspaceDir, "config.toml"), content, 0644)
		if err != nil {
			return false, false, err
		}
	}

	return updateMcp, updateSkill, nil
}

func (s *Service) isUserWorkspaceExists(auth *utils.Auth) (string, bool) {
	path := filepath.Join(WorkspaceDir, UserWorkDirName, strconv.Itoa(auth.ID))
	if _, err := os.Stat(path); err != nil {
		return path, false
	}
	if _, err := os.Stat(filepath.Join(path, "config.toml")); err != nil {
		return path, false
	}
	return path, true
}

func (s *Service) getWorkspaceConfig(addMcps, delMcps []*schema.Tool) (*codex.Config, error) {
	path := filepath.Join(WorkspaceDir, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg *codex.Config
	if err = toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	for _, item := range delMcps {
		delete(cfg.McpServers, item.Name)
	}
	for _, item := range addMcps {
		info := codex.McpServer{
			Url:         item.Url,
			HttpHeaders: make(map[string]string),
		}
		if item.Token != "" {
			info.HttpHeaders["Authorization"] = "Bearer " + item.Token
		} else {
			info.BearerTokenEnvVar = "MAGENT_USER_TOKEN"
		}
		cfg.McpServers[item.Name] = info
	}
	return cfg, nil
}

func (s *Service) getEndpoint(id int) (string, error) {
	podName := fmt.Sprintf("codex-%d", id)
	endpoint := fmt.Sprintf("ws://%s:8000", podName)
	return endpoint, nil
}
