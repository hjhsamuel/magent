package service

import (
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store"
)

type Service struct {
	node string

	rb    *ringbuffer.Manager
	cm    *codex.Server
	store *store.Store
}
