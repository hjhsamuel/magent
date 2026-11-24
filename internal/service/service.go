package service

import (
	"sync"

	"github.com/hjhsamuel/magent/config"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/container"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store"
	"github.com/hjhsamuel/magent/pkg/llm"
	"github.com/sirupsen/logrus"
)

type Service struct {
	wsToken      string
	runtimeImage string

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

	if err := s.initCron(); err != nil {
		return err
	}

	return nil
}

func (s *Service) Close() error {
	close(s.exit)

	s.cm.Close()
	s.rb.Close()

	s.wg.Wait()
	return nil
}

func NewService(cfg *config.Config) (*Service, error) {
	s := &Service{
		wsToken:      cfg.Runtime.Token,
		runtimeImage: cfg.Runtime.Image,
		rb:           ringbuffer.NewManager(),
		exit:         make(chan struct{}),
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
