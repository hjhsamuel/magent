package service

import (
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/container"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store"
)

type Service struct {
	node    string
	wsToken string

	rb    *ringbuffer.Manager
	cm    *codex.Manager
	k8s   *container.Manager
	store *store.Store
}
