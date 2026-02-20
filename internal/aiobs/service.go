// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package aiobs provides AI observability services for monitoring AI agent activity.
package aiobs

import (
	"context"
	"sync"

	"github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/aiobs/watcher"
	"github.com/noldarim/noldarim/internal/logger"
)

var log = logger.GetLogger("aiobs")

// Service provides AI observability capabilities.
type Service struct {
	watchers map[string]*watcher.TranscriptWatcher
	mu       sync.RWMutex
}

// NewService creates a new AI observability service.
func NewService() *Service {
	return &Service{
		watchers: make(map[string]*watcher.TranscriptWatcher),
	}
}

// WatchTranscript starts watching a transcript file for a given task.
// Events are emitted on the returned channel.
func (s *Service) WatchTranscript(ctx context.Context, taskID string, cfg watcher.Config) (<-chan types.ParsedEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing watcher if any
	if existing, ok := s.watchers[taskID]; ok {
		existing.Stop()
		delete(s.watchers, taskID)
	}

	w, err := watcher.NewTranscriptWatcher(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := w.Start(); err != nil {
		return nil, err
	}

	s.watchers[taskID] = w
	log.Info().Str("taskID", taskID).Str("file", cfg.FilePath).Msg("Started transcript watcher")

	return w.Events(), nil
}

// StopWatcher stops the watcher for a given task.
func (s *Service) StopWatcher(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, ok := s.watchers[taskID]; ok {
		w.Stop()
		delete(s.watchers, taskID)
		log.Info().Str("taskID", taskID).Msg("Stopped transcript watcher")
	}
}

// StopAll stops all watchers.
func (s *Service) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for taskID, w := range s.watchers {
		w.Stop()
		delete(s.watchers, taskID)
	}
	log.Info().Msg("Stopped all transcript watchers")
}

// GetWatcherStats returns stats for a task's watcher.
func (s *Service) GetWatcherStats(taskID string) (watcher.WatcherStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if w, ok := s.watchers[taskID]; ok {
		return w.Stats(), true
	}
	return watcher.WatcherStats{}, false
}
