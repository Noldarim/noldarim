// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/go-chi/chi/v5"
)

// Server is the REST + WebSocket API server.
type Server struct {
	httpServer  *http.Server
	broadcaster *EventBroadcaster
}

// New creates and wires up the API server. It does NOT start listening —
// call Run() for that.
func New(
	cfg *config.ServerConfig,
	eventChan <-chan protocol.Event,
	dataService *services.DataService,
	gitMgr *services.GitServiceManager,
	pipeline *services.PipelineService,
	agentDefaults AgentDefaultsResponse,
) *Server {
	registry := NewClientRegistry()
	broadcaster := NewEventBroadcaster(eventChan, registry)
	handlers := NewHandlers(broadcaster, dataService, gitMgr, pipeline, agentDefaults)

	r := chi.NewRouter()

	// Global middleware
	r.Use(Recovery)
	r.Use(RequestID)
	r.Use(Logger)
	r.Use(CORS(cfg.AllowedOrigins))
	r.Use(MaxBodySize(1 << 20)) // 1 MB default

	// REST routes
	r.Route("/api/v1", func(r chi.Router) {
		// Projects
		r.Get("/projects", handlers.GetProjects)
		r.Post("/projects", handlers.CreateProject)
		r.Get("/agent/defaults", handlers.GetAgentDefaults)

		// Project sub-resources
		r.Route("/projects/{id}", func(r chi.Router) {
			r.Get("/tasks", handlers.GetTasks)
			r.Post("/tasks", handlers.CreateTask)
			r.Get("/commits", handlers.GetCommits)
			r.Get("/pipelines", handlers.GetPipelineRuns)
			r.Post("/pipelines", handlers.StartPipeline)

			// Task sub-resources
			r.Post("/tasks/{taskId}/toggle", handlers.ToggleTask)
			r.Delete("/tasks/{taskId}", handlers.DeleteTask)
			r.Get("/tasks/{taskId}/activity", handlers.GetAIActivity)
		})

		// Pipeline operations
		r.Get("/pipelines/{runId}", handlers.GetPipelineRun)
		r.Get("/pipelines/{runId}/activity", handlers.GetPipelineRunAIActivity)
		r.Post("/pipelines/{runId}/cancel", handlers.CancelPipeline)
	})

	// WebSocket
	r.Get("/ws", HandleWebSocket(registry, cfg.AllowedOrigins))

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	return &Server{
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		broadcaster: broadcaster,
	}
}

// Run starts the event broadcaster goroutine and the HTTP server.
// Blocks until the server is shut down or the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	go func() {
		const maxRetries = 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			func() {
				defer func() {
					if r := recover(); r != nil {
						getLog().Error().Interface("panic", r).Int("attempt", attempt).Msg("Event broadcaster panic")
					}
				}()
				s.broadcaster.Run(ctx)
			}()

			// Normal return (context cancelled) — exit without retry.
			if ctx.Err() != nil {
				return
			}

			if attempt < maxRetries {
				getLog().Warn().Int("attempt", attempt).Msg("Restarting event broadcaster after panic")
				time.Sleep(1 * time.Second)
			}
		}
		getLog().Error().Msg("Event broadcaster exhausted retries - events will no longer be dispatched")
	}()

	getLog().Info().Str("addr", s.httpServer.Addr).Msg("API server listening")
	err := s.httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
