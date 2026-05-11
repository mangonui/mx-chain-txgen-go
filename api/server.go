package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mangonui/mx-chain-txgen-go/config"
	"github.com/mangonui/mx-chain-txgen-go/scenarios"
	"github.com/mangonui/mx-chain-txgen-go/stats"
)

// Server is the txgen's HTTP listener. Mounts the upstream-compatible
// /transaction/send-multiple endpoint and a /status diagnostic endpoint.
type Server struct {
	cfg    config.ServerConfig
	srv    *http.Server
	engine *gin.Engine
}

// New constructs a server bound to the given port with the scenario
// registry, shared component bundle, and (optionally) a stats sampler
// wired in. Sampler may be nil — when nil, /stats returns an empty
// report and sendMultiple skips recording.
func New(cfg config.ServerConfig, registry *scenarios.Registry, comp *scenarios.Components, sampler *stats.Sampler) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	h := &handler{registry: registry, comp: comp, sampler: sampler}
	engine.POST("/transaction/send-multiple", h.sendMultiple)
	engine.GET("/status", h.status)
	engine.GET("/stats", h.stats)

	addr := ":" + strconv.Itoa(cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return &Server{cfg: cfg, srv: srv, engine: engine}
}

// Start runs the server until the context is cancelled or an unrecoverable
// listener error occurs. The returned error is nil on graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("listen on %s: %w", s.srv.Addr, err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	}
}
