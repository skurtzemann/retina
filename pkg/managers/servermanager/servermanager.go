// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package servermanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/server"
	"go.uber.org/zap"
)

type HTTPServer struct {
	l      *log.ZapLogger
	host   string
	port   int
	router *server.Server
	cache  *cache.Cache
}

func NewHTTPServer(
	host string, port int,
) *HTTPServer {
	logger := log.Logger().Named("http-server")
	return &HTTPServer{
		l:    logger,
		host: host,
		port: port,
	}
}

func (s *HTTPServer) SetCache(c *cache.Cache) {
	if c == nil {
		s.l.Warn("HTTP server cache is nil")
	} else {
		s.l.Info("HTTP server cache connected", zap.String("cache_name", c.GetName()))
	}
	s.cache = c
}

func (s *HTTPServer) handleCacheDebug(w http.ResponseWriter, r *http.Request) {
	if s.cache == nil {
		http.Error(w, "cache not initialized", http.StatusServiceUnavailable)
		return
	}

	stats := s.cache.GetStats()
	callCount := s.cache.GetGetZoneByPodIPCallCount()

	response := map[string]interface{}{
		"cache_stats":              stats,
		"get_zone_by_pod_ip_calls": callCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *HTTPServer) Init() error {
	s.l.Info("Initializing HTTP server ...")
	rt := server.New(s.l)
	rt.SetupHandlers()

	// Register debug endpoints
	rt.AddHandler("/debug/cache", s.handleCacheDebug)

	s.router = rt
	s.l.Info("HTTP server initialized...")
	return nil
}

func (s *HTTPServer) Start(ctx context.Context) error {
	s.l.Info("Starting HTTP server ...", zap.String("host", s.host), zap.Int("port", s.port))
	return s.router.Start(ctx, fmt.Sprintf("%s:%d", s.host, s.port))
}
