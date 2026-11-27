package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hjhsamuel/cron"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Service) GetCronList(auth *utils.Auth, offset string, limit int64, status *int) ([]*schema.Cron, error) {
	filter := bson.M{"creator": auth.ID}
	if offset != "" {
		cid, err := bson.ObjectIDFromHex(offset)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$gt": cid}
	}
	if status != nil {
		filter["status"] = *status
	}

	return s.store.GetCronJobs(
		context.Background(),
		filter,
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(limit),
	)
}

func (s *Service) DeleteCronJob(auth *utils.Auth, id string) error {
	cid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	err = s.store.DeleteCronJob(context.Background(), bson.M{"_id": cid, "creator": auth.ID})
	if err != nil {
		return err
	}

	s.cron.RemoveCronJob(cid.Hex())
	return nil
}

func (s *Service) AddCronJob(auth *utils.Auth, name string, spec string, content string) (bson.ObjectID, error) {
	_, err := s.cron.Parse(spec)
	if err != nil {
		return bson.ObjectID{}, err
	}

	id, err := s.store.AddCronJob(context.Background(), &schema.Cron{
		Name:      name,
		Schedule:  spec,
		Content:   content,
		Status:    schema.CronStateOnline,
		Creator:   auth.ID,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return bson.ObjectID{}, err
	}

	err = s.cron.AddCronJob(id.Hex(), spec, &CronUserJob{
		s:       s,
		uid:     auth.ID,
		content: content,
	})
	if err != nil {
		_ = s.store.DeleteCronJob(context.Background(), bson.M{"_id": id})
		return bson.ObjectID{}, err
	}

	return id, nil
}

func (s *Service) UpdateCronJob(
	auth *utils.Auth,
	id string,
	name *string,
	spec *string,
	content *string,
	status *int,
) error {
	cid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	set := bson.M{}
	if name != nil {
		set["name"] = *name
	}
	if spec != nil {
		if _, err := s.cron.Parse(*spec); err != nil {
			return err
		}
		set["schedule"] = *spec
	}
	if content != nil {
		set["content"] = *content
	}
	if status != nil {
		state := schema.CronState(*status)
		if state != schema.CronStateOnline && state != schema.CronStateOffline {
			return errors.New("invalid cron status")
		}
		set["status"] = state
	}
	if len(set) == 0 {
		return errors.New("no cron fields to update")
	}

	previous, err := s.store.UpdateCronJob(
		context.Background(),
		bson.M{"_id": cid, "creator": auth.ID},
		bson.M{"$set": set},
	)
	if err != nil {
		return err
	}

	stateChanged := status != nil && schema.CronState(*status) != previous.Status
	if stateChanged && schema.CronState(*status) == schema.CronStateOffline {
		s.cron.RemoveCronJob(cid.Hex())
		return nil
	}

	scheduleChanged := spec != nil && *spec != previous.Schedule
	contentChanged := content != nil && *content != previous.Content
	if !stateChanged && (previous.Status == schema.CronStateOffline || (!scheduleChanged && !contentChanged)) {
		return nil
	}

	if spec != nil {
		previous.Schedule = *spec
	}
	if content != nil {
		previous.Content = *content
	}
	s.cron.RemoveCronJob(cid.Hex())
	return s.cron.AddCronJob(cid.Hex(), previous.Schedule, &CronUserJob{
		s:       s,
		uid:     auth.ID,
		content: previous.Content,
	})
}

type CronService struct {
	c     *cron.Cron
	entry sync.Map
}

func (c *CronService) Start() {
	c.c.Start()
}

func (c *CronService) Close() {
	ctx := c.c.Stop()
	<-ctx.Done()
}

func (c *CronService) RemoveCronJob(key string) {
	v, loaded := c.entry.LoadAndDelete(key)
	if !loaded {
		return
	}
	id := v.(cron.EntryID)
	c.c.Remove(id)
}

func (c *CronService) Parse(spec string) (cron.Schedule, error) {
	return cron.Parse(spec)
}

func (c *CronService) AddCronJob(key string, spec string, job cron.Job) error {
	_, ok := c.entry.Load(key)
	if ok {
		return errors.New("job already exists")
	}

	sch, err := c.Parse(spec)
	if err != nil {
		return err
	}
	id := c.c.Schedule(sch, job)
	c.entry.Store(key, id)
	return nil
}

func NewCronService() *CronService {
	c := cron.New()
	return &CronService{c: c}
}

type CronOnlineTools struct {
	s *Service
}

func (c *CronOnlineTools) Run() {
	tools, err := c.s.store.FindTools(
		context.Background(),
		bson.M{"status": bson.M{"$in": []schema.ToolState{
			schema.ToolStatusReady,
			schema.ToolStateUninstall,
		}}},
	)
	if err != nil {
		logrus.WithError(err).Error("get tools")
		return
	}

	updateMcp, updateSkill, err := c.s.updateWorkspaceConfig(tools...)
	if err != nil {
		logrus.WithError(err).Error("update workspace")
		return
	}

	if err = c.s.updateAllUserWorkspace(updateMcp, updateSkill); err != nil {
		logrus.WithError(err).Error("update user workspace")
	}

	var (
		onlineIds  = make([]bson.ObjectID, 0)
		offlineIds = make([]bson.ObjectID, 0)
	)
	for _, item := range tools {
		switch item.Status {
		case schema.ToolStatusReady:
			onlineIds = append(onlineIds, item.ID)
		case schema.ToolStateUninstall:
			offlineIds = append(offlineIds, item.ID)
		}
	}
	if len(onlineIds) != 0 {
		_, _ = c.s.store.UpdateTool(
			context.Background(),
			bson.M{"_id": bson.M{"$in": onlineIds}},
			bson.M{"$set": bson.M{
				"status":     schema.ToolStateOnline,
				"updated_at": time.Now(),
			}},
		)
	}
	if len(offlineIds) != 0 {
		_, _ = c.s.store.RemoveTool(
			context.Background(),
			bson.M{"_id": bson.M{"$in": offlineIds}},
		)
	}
}

type CronUserJob struct {
	s       *Service
	running atomic.Bool
	uid     int
	content string
}

func (c *CronUserJob) Run() {
	if c.running.Load() {
		return
	}

	c.running.Store(true)
	defer c.running.Store(false)

	// 连接 runtime
	ur, err := c.s.cm.CreateOrGet(c.uid, c.s.getWebsocketHeaders())
	if err != nil {
		logrus.WithError(err).Error("runtime connect")
		return
	}
	r, err := ur.OpenRuntime(context.Background(), "")
	if err != nil {
		logrus.WithError(err).Error("create runtime session")
		return
	}

	// ringbuffer
	buffer, err := c.s.rb.Create(r.ThreadID())
	if err != nil {
		_ = r.Close()
		return
	}
	producer, err := buffer.ConnectProducer()
	if err != nil {
		_ = r.Close()
		return
	}
	defer producer.Close()

	// 创建会话
	sid, err := c.s.store.CreateSession(context.Background(), &schema.Session{
		Name:      "New Chat",
		User:      c.uid,
		Status:    schema.SessionStateInit,
		RunId:     r.ThreadID(),
		CreatedAt: time.Now(),
	})
	if err != nil {
		logrus.WithError(err).Error("create session")
		_ = r.Close()
		return
	}

	// 开启轮次
	turnId, index, err := c.s.store.StartTurn(context.Background(), sid, c.content)
	if err != nil {
		logrus.WithError(err).Error("start turn")
		_ = r.Close()
		return
	}
	rsp, err := r.TurnStart(context.Background(), codex.TurnStartParams{
		ThreadID: r.ThreadID(),
		Input:    []codex.TextInput{{Type: "text", Text: c.content}},
	})
	if err != nil {
		logrus.WithError(err).Error("start turn")
		_ = r.Close()
		_ = c.s.store.TurnComplete(
			context.Background(),
			r.ThreadID(),
			bson.M{"_id": turnId},
			&schema.Message{
				Role:      schema.MessageRoleModel,
				Content:   err.Error(),
				CreatedAt: time.Now(),
			},
		)
		return
	}

	c.s.turnStart(sid, index, r, producer, c.content)

	_ = c.s.store.UpdateTurn(
		context.Background(),
		bson.M{"_id": turnId},
		bson.M{"$set": bson.M{"turn_id": rsp.Turn.ID}},
	)
}
