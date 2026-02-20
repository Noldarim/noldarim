// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/tui/components/elapsedtimer"
)

func main() {
	startTime := loadStartTime()
	m := elapsedtimer.New().StartFrom(startTime)
	fmt.Println(m.View())
}

func loadStartTime() time.Time {
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		return mockStartTime()
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return mockStartTime()
	}
	defer dataService.Close()

	ctx := context.Background()
	run, err := dataService.GetLatestPipelineRun(ctx)
	if err != nil || run == nil {
		return mockStartTime()
	}

	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		return *run.StartedAt
	}

	return mockStartTime()
}

func mockStartTime() time.Time {
	return time.Now().Add(-2*time.Minute - 34*time.Second)
}
