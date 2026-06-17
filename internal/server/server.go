// Package server wires the gin HTTP router serving the API, GUI and /metrics.
package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bonukr/phantom-exporter/internal/metrics"
	"github.com/bonukr/phantom-exporter/internal/store"
)

// Server holds dependencies shared by the HTTP handlers.
type Server struct {
	store  *store.Store
	gen    *metrics.Generator
	log    *slog.Logger
	stats  *Stats
	web    fs.FS
	engine *gin.Engine
}

// New builds a Server and configures all routes. webFS holds the GUI assets.
func New(st *store.Store, gen *metrics.Generator, log *slog.Logger, webFS fs.FS) *Server {
	gin.SetMode(gin.ReleaseMode)
	s := &Server{
		store: st,
		gen:   gen,
		log:   log,
		stats: NewStats(),
		web:   webFS,
	}

	r := gin.New()
	r.Use(gin.Recovery(), s.requestLogger())
	s.routes(r)
	s.engine = r
	return s
}

func (s *Server) routes(r *gin.Engine) {
	// GUI (static assets embedded under web/).
	r.GET("/", s.serveIndex)
	r.StaticFS("/static", http.FS(s.web))

	// Prometheus scrape endpoint, one path per metric group.
	r.GET("/metrics/:path", s.handleScrape)

	api := r.Group("/api")
	{
		api.GET("/status", s.handleStatus)
		api.GET("/stats", s.handleStats)

		api.GET("/groups", s.listGroups)
		api.POST("/groups", s.createGroup)
		api.GET("/groups/:id", s.getGroup)
		api.PUT("/groups/:id", s.updateGroup)
		api.DELETE("/groups/:id", s.deleteGroup)

		api.POST("/groups/:id/metrics", s.createMetric)
		api.GET("/metrics-def/:mid", s.getMetric)
		api.PUT("/metrics-def/:mid", s.updateMetric)
		api.DELETE("/metrics-def/:mid", s.deleteMetric)
		api.POST("/metrics-def/:mid/value", s.setMetricValue)
	}
}

// requestLogger is a gin middleware that logs each request via slog.
func (s *Server) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.log.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client", c.ClientIP(),
		)
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down.
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.engine}

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.log.Info("shutting down http server")
		return srv.Shutdown(shutdownCtx)
	}
}
