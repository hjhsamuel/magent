package service

import (
	"github.com/hjhsamuel/magent/internal/runtime"
	"github.com/hjhsamuel/magent/internal/store"
)

type Service struct {
	node    string
	workdir string

	manager *runtime.Manager
	store   *store.Store
}
