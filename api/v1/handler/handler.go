package handler

import "github.com/hjhsamuel/magent/internal/service"

type Api struct {
	srv *service.Service
}

func NewApi(srv *service.Service) *Api {
	return &Api{srv: srv}
}
