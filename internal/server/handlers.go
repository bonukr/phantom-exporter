package server

import (
	"errors"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/bonukr/phantom-exporter/internal/metrics"
	"github.com/bonukr/phantom-exporter/internal/store"
)

// ---- GUI ----

func (s *Server) serveIndex(c *gin.Context) {
	data, err := s.readWeb("index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "index not found")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// ---- Prometheus scrape ----

func (s *Server) handleScrape(c *gin.Context) {
	path := c.Param("path")
	g, err := s.store.GetGroupByPath(c.Request.Context(), path)
	if errors.Is(err, store.ErrNotFound) {
		c.String(http.StatusNotFound, "# unknown metric group: %s\n", path)
		return
	}
	if err != nil {
		s.log.Error("scrape failed", "path", path, "error", err)
		c.String(http.StatusInternalServerError, "# error\n")
		return
	}
	s.stats.RecordScrape(path)
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8",
		[]byte(metrics.Render(g, s.gen)))
}

// ---- Monitoring ----

func (s *Server) handleStatus(c *gin.Context) {
	ctx := c.Request.Context()
	dbOK := s.store.Ping(ctx) == nil
	count, _ := s.store.CountMetrics(ctx)
	groups, _ := s.store.ListGroups(ctx)
	c.JSON(http.StatusOK, gin.H{
		"dbConnected":   dbOK,
		"metricCount":   count,
		"groupCount":    len(groups),
		"uptimeSeconds": s.stats.Snapshot().UptimeSeconds,
	})
}

func (s *Server) handleStats(c *gin.Context) {
	c.JSON(http.StatusOK, s.stats.Snapshot())
}

// ---- Groups ----

func (s *Server) listGroups(c *gin.Context) {
	groups, err := s.store.ListGroups(c.Request.Context())
	if err != nil {
		s.fail(c, err)
		return
	}
	if groups == nil {
		groups = []metrics.Group{}
	}
	c.JSON(http.StatusOK, groups)
}

type groupReq struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (s *Server) createGroup(c *gin.Context) {
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		s.badRequest(c, err)
		return
	}
	if err := metrics.ValidatePath(req.Path); err != nil {
		s.badRequest(c, err)
		return
	}
	g, err := s.store.CreateGroup(c.Request.Context(), req.Path, req.Name)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, g)
}

func (s *Server) getGroup(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	g, err := s.store.GetGroup(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func (s *Server) updateGroup(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	var req groupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		s.badRequest(c, err)
		return
	}
	if err := metrics.ValidatePath(req.Path); err != nil {
		s.badRequest(c, err)
		return
	}
	if err := s.store.UpdateGroup(c.Request.Context(), id, req.Path, req.Name); err != nil {
		s.failNotFound(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) deleteGroup(c *gin.Context) {
	id, err := idParam(c, "id")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	if err := s.store.DeleteGroup(c.Request.Context(), id); err != nil {
		s.failNotFound(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- Metrics ----

func (s *Server) createMetric(c *gin.Context) {
	groupID, err := idParam(c, "id")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	var m metrics.Metric
	if err := c.ShouldBindJSON(&m); err != nil {
		s.badRequest(c, err)
		return
	}
	m.GroupID = groupID
	m.Override = nil
	if m.Labels == nil {
		m.Labels = []metrics.Label{}
	}
	if err := m.Validate(); err != nil {
		s.badRequest(c, err)
		return
	}
	if err := s.store.CreateMetric(c.Request.Context(), &m); err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (s *Server) getMetric(c *gin.Context) {
	id, err := idParam(c, "mid")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	m, err := s.store.GetMetric(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "metric not found"})
		return
	}
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func (s *Server) updateMetric(c *gin.Context) {
	id, err := idParam(c, "mid")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	var m metrics.Metric
	if err := c.ShouldBindJSON(&m); err != nil {
		s.badRequest(c, err)
		return
	}
	m.ID = id
	if m.Labels == nil {
		m.Labels = []metrics.Label{}
	}
	if err := m.Validate(); err != nil {
		s.badRequest(c, err)
		return
	}
	if err := s.store.UpdateMetric(c.Request.Context(), &m); err != nil {
		s.failNotFound(c, err)
		return
	}
	s.gen.Reset(id)
	c.Status(http.StatusNoContent)
}

func (s *Server) deleteMetric(c *gin.Context) {
	id, err := idParam(c, "mid")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	if err := s.store.DeleteMetric(c.Request.Context(), id); err != nil {
		s.failNotFound(c, err)
		return
	}
	s.gen.Reset(id)
	c.Status(http.StatusNoContent)
}

type valueReq struct {
	// Value is the override; null clears the override and resumes simulation.
	Value *float64 `json:"value"`
}

func (s *Server) setMetricValue(c *gin.Context) {
	id, err := idParam(c, "mid")
	if err != nil {
		s.badRequest(c, err)
		return
	}
	var req valueReq
	if err := c.ShouldBindJSON(&req); err != nil {
		s.badRequest(c, err)
		return
	}
	if err := s.store.SetOverride(c.Request.Context(), id, req.Value); err != nil {
		s.failNotFound(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ---- helpers ----

func idParam(c *gin.Context, key string) (int64, error) {
	return strconv.ParseInt(c.Param(key), 10, 64)
}

func (s *Server) readWeb(name string) ([]byte, error) {
	return fs.ReadFile(s.web, name)
}

func (s *Server) badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (s *Server) fail(c *gin.Context, err error) {
	s.log.Error("request failed", "path", c.Request.URL.Path, "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func (s *Server) failNotFound(c *gin.Context, err error) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	s.fail(c, err)
}
