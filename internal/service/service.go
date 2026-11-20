package service

import (
	"github.com/hjhsamuel/magent/config"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/container"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store"
	"github.com/sirupsen/logrus"
)

type Service struct {
	wsToken string

	rb    *ringbuffer.Manager
	cm    *codex.Manager
	k8s   *container.Manager
	store *store.Store
}

func NewService(cfg *config.Config) (*Service, error) {
	s := &Service{
		wsToken: cfg.Runtime.Token,
		rb:      ringbuffer.NewManager(),
	}

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

	return s, nil
}
