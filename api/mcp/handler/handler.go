package handler

import (
	"net/http"

	"github.com/hjhsamuel/magent/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Api struct {
	mcpSvc *mcp.Server
	srv    *service.Service
}

func (a *Api) init() {
	a.initCron()
}

func NewHandler(srv *service.Service) *mcp.StreamableHTTPHandler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:        "cronjob",
		Version:     "1.0.0",
		Description: "The cronjob manager of agent",
	}, nil)

	api := &Api{
		srv:    srv,
		mcpSvc: server,
	}
	server.AddReceivingMiddleware(authenticateToolCall)
	api.init()

	return mcp.NewStreamableHTTPHandler(
		func(req *http.Request) *mcp.Server {
			return api.mcpSvc
		},
		&mcp.StreamableHTTPOptions{
			Stateless: true,
		},
	)
}
