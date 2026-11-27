package service

import (
	"context"
	"sync"

	"github.com/hjhsamuel/magent/config"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/container"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/hjhsamuel/magent/pkg/llm"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
	wsToken    string
	runtimePod config.RuntimePod

	cron  *CronService
	topic *llm.LLM
	rb    *ringbuffer.Manager
	cm    *codex.Manager
	k8s   *container.Manager
	store *store.Store

	exit chan struct{}
	wg   sync.WaitGroup
}

func (s *Service) init() error {
	if err := s.load(); err != nil {
		return err
	}

	if err := s.initCronJobs(); err != nil {
		return err
	}

	return nil
}

func (s *Service) initCronJobs() error {
	err := s.cron.AddCronJob("_online_tools", "@every 1m", &CronOnlineTools{})
	if err != nil {
		return err
	}

	objs, err := s.store.GetCronJobs(context.Background(), bson.M{"status": schema.CronStateOnline})
	if err != nil {
		return err
	}
	for _, item := range objs {
		err = s.cron.AddCronJob(item.ID.Hex(), item.Schedule, &CronUserJob{
			s:       s,
			uid:     item.Creator,
			content: item.Content,
		})
		if err != nil {
			logrus.WithError(err).WithField("id", item.ID.Hex()).Error("add user cron job")
		}
	}

	s.cron.Start()

	return nil
}

func (s *Service) Close() error {
	close(s.exit)

	s.cron.Close()

	s.cm.Close()
	s.rb.Close()

	s.wg.Wait()
	return nil
}

func NewService(cfg *config.Config) (*Service, error) {
	s := &Service{
		wsToken:    cfg.Runtime.Token,
		runtimePod: cfg.Runtime.Pod,
		cron:       NewCronService(),
		rb:         ringbuffer.NewManager(),
		exit:       make(chan struct{}),
	}

	client, err := llm.NewLLM(cfg.Runtime.Topic.Url, cfg.Runtime.Topic.ApiKey, cfg.Runtime.Topic.Model)
	if err != nil {
		return nil, err
	}
	s.topic = client

	k8s, err := container.NewK8sManager()
	if err != nil {
		return nil, err
	}
	s.k8s = k8s

	dao, err := store.NewStore(cfg.Store, logrus.StandardLogger())
	if err != nil {
		return nil, err
	}
	s.store = dao

	cm, err := codex.NewRuntimeManager(s.getEndpoint)
	if err != nil {
		return nil, err
	}
	s.cm = cm

	if err = s.init(); err != nil {
		return nil, err
	}

	return s, nil
}
