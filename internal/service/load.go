package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	corev1 "k8s.io/api/core/v1"
)

func (s *Service) load() error {
	objs, err := s.store.FindSession(
		context.Background(),
		bson.M{"status": bson.M{"$ne": schema.SessionStateCompleted}},
	)
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return nil
	}

	pods, err := s.k8s.GetPodList(context.Background(), s.k8s.Namespace(), "agent=true")
	if err != nil {
		return err
	}
	podMap := make(map[string]corev1.PodPhase)
	for _, item := range pods {
		podMap[item.Name] = item.Status.Phase
	}

	userSession := make(map[int][]*schema.Session)
	for _, item := range objs {
		if _, ok := userSession[item.User]; !ok {
			userSession[item.User] = make([]*schema.Session, 0)
		}
		userSession[item.User] = append(userSession[item.User], item)
	}

	for userId, sessionObjs := range userSession {
		podName := fmt.Sprintf("codex-%d", userId)
		if v, ok := podMap[podName]; !ok || v != corev1.PodRunning {
			continue
		} else {
			if err = s.syncSession(userId, sessionObjs...); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) syncSession(userId int, objs ...*schema.Session) error {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+s.wsToken)
	ur, err := s.cm.CreateOrGet(userId, headers)
	if err != nil {
		return err
	}

	var (
		ids        = make([]string, 0, len(objs))
		sessionMap = make(map[string]*schema.Session)
	)
	for _, item := range objs {
		ids = append(ids, item.RunId)
		sessionMap[item.RunId] = item
	}
	runMap, err := ur.ThreadStatuses(context.Background(), ids...)
	if err != nil {
		return err
	}

	doneIds := make([]string, 0)
	for id, status := range runMap {
		switch status.Type {
		case codex.ThreadStatusNotLoaded, codex.ThreadStatusIdle, codex.ThreadStatusSystemError:
			doneIds = append(doneIds, id)
		case codex.ThreadStatusActive:
			item, ok := sessionMap[id]
			if !ok {
				continue
			}
			buffer, err := s.rb.CreateOrGet(id)
			if err != nil {
				continue
			}
			producer, err := buffer.ConnectProducer()
			if err != nil {
				continue
			}
			r, err := ur.OpenRuntime(context.Background(), id)
			if err != nil {
				producer.Close()
				continue
			}
			go s.turnStart(item.ID, item.Seq-1, r, producer, "")
		}
	}

	if len(doneIds) != 0 {
		turnMap, err := ur.ThreadTurns(context.Background(), 1, doneIds...)
		if err != nil {
			return err
		}
		for runId, turns := range turnMap {
			if len(turns) == 0 {
				continue
			}
			sessionObj, ok := sessionMap[runId]
			if !ok {
				continue
			}
			turn := turns[0]
			// 异常
			if turn.Error != nil {
				_ = s.store.UpdateTurn(
					context.Background(),
					bson.M{"session": sessionObj.ID, "index": sessionObj.Seq - 1},
					bson.M{
						"$set": bson.M{"turn_id": turn.ID},
						"$push": bson.M{"messages": &schema.Message{
							Role:      schema.MessageRoleModelError,
							Content:   turn.Error.Message,
							CreatedAt: time.Now(),
						}},
					},
				)
				continue
			}
			// 正常
			messages := make([]*schema.Message, 0)
			for _, item := range turn.Items {
				var info codex.SummaryItem
				if err = json.Unmarshal(item, &info); err != nil {
					continue
				}
				if info.Type != codex.AgentMessage {
					continue
				}

				message := &schema.Message{
					Role:    schema.MessageRoleModel,
					Content: info.Text,
				}
				if info.CompletedAtMs != 0 {
					message.CreatedAt = time.UnixMilli(info.CompletedAtMs)
				} else {
					message.CreatedAt = time.Now()
				}
				messages = append(messages, message)
			}
			_ = s.store.UpdateTurn(
				context.Background(),
				bson.M{"session": sessionObj.ID, "index": sessionObj.Seq - 1},
				bson.M{
					"$set":  bson.M{"turn_id": turn.ID},
					"$push": bson.M{"messages": bson.M{"$each": messages}},
				},
			)

			// 会话
			_ = s.store.UpdateSession(
				context.Background(),
				bson.M{"_id": sessionObj.ID},
				bson.M{"$set": bson.M{"status": schema.SessionStateCompleted, "updated_at": time.Now()}},
			)
		}
	}

	return nil
}
