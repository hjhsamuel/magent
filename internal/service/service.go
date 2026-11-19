package service

import (
	"github.com/hjhsamuel/magent/internal/ringbuffer"
	"github.com/hjhsamuel/magent/internal/store"
)

type Service struct {
	node string

	rb    *ringbuffer.Manager
	cm    *RuntimeManager
	store *store.Store
}
