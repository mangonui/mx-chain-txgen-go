package api

import (
	"context"
	"fmt"
	"log"
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
	engine.Use(loggingMiddleware())

	h := &handler{registry: registry, comp: comp, sampler: sampler}
	engine.POST("/transaction/send-multiple", h.sendMultiple)
	engine.GET("/status", h.status)
	engine.GET("/stats", h.stats)
	engine.GET("/healthz", h.healthz)

	addr := ":" + strconv.Itoa(cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return &Server{cfg: cfg, srv: srv, engine: engine}
}

// Start runs the server until the context is cancelled or an
// unrecoverable listener error occurs. The returned error is nil on
// graceful shutdown.
//
// On ctx.Done(), Shutdown is called with the configured
// ShutdownTimeoutSeconds budget. The handler-level request mutex
// ensures at most one /transaction/send-multiple is in flight at any
// time, so the realistic worst-case drain is one full scenario run.
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
		timeout := time.Duration(s.cfg.ShutdownTimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 15 * time.Second
		}
		log.Printf("shutdown: waiting up to %s for in-flight scenario to complete", timeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	}
}
