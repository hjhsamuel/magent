package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/magent/api"
	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/api/utils/middleware"
	"github.com/hjhsamuel/magent/config"
	"github.com/hjhsamuel/magent/internal/service"
	"github.com/sirupsen/logrus"
)

func Start() error {
	if err := config.Init(); err != nil {
		return err
	}

	cfg := config.GetConfig()

	svc, err := startServer(cfg)
	if err != nil {
		return err
	}
	defer svc.Close()

	httpSrv, err := startWeb(cfg, svc)
	if err != nil {
		return err
	}

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	<-exit
	logrus.Infof("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = httpSrv.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	}

	return nil
}

func startServer(cfg *config.Config) (*service.Service, error) {
	return service.NewService(cfg)
}

func startWeb(cfg *config.Config, srv *service.Service) (*http.Server, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger)
	api.Route(srv, r)

	utils.SetCookieKey(cfg.Server.Header)
	utils.SetJwtSalt(cfg.Server.Salt)

	httpSrv := http.Server{
		Addr:    net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler: r,
	}

	listener, err := net.Listen("tcp", httpSrv.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen HTTP server on %s: %w", httpSrv.Addr, err)
	}

	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Error("HTTP server stopped unexpectedly")
		}
	}()

	logrus.Infof("Server started, listening at: %s", listener.Addr())

	return &httpSrv, nil
}
