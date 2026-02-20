// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/tui/components/stepprogress"
)

func main() {
	steps := loadSteps()
	component := stepprogress.New().SetSteps(steps).SetWidth(20)
	fmt.Println(component.View())
}

func loadSteps() []stepprogress.Step {
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		return mockSteps()
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return mockSteps()
	}
	defer dataService.Close()

	ctx := context.Background()
	run, err := dataService.GetLatestPipelineRun(ctx)
	if err != nil || run == nil || len(run.StepResults) == 0 {
		return mockSteps()
	}

	pipeline, _ := dataService.GetPipeline(ctx, run.PipelineID)

	steps := make([]stepprogress.Step, len(run.StepResults))
	for i, result := range run.StepResults {
		name := result.StepID
		if pipeline != nil {
			for _, def := range pipeline.Steps {
				if def.StepID == result.StepID {
					name = def.Name
					break
				}
			}
		}
		steps[i] = stepprogress.Step{
			Name:   name,
			Status: convertStatus(result.Status),
		}
	}

	return steps
}

func convertStatus(s models.StepStatus) stepprogress.StepStatus {
	switch s {
	case models.StepStatusCompleted:
		return stepprogress.StatusCompleted
	case models.StepStatusRunning:
		return stepprogress.StatusRunning
	case models.StepStatusFailed:
		return stepprogress.StatusFailed
	case models.StepStatusSkipped:
		return stepprogress.StatusSkipped
	default:
		return stepprogress.StatusPending
	}
}

func mockSteps() []stepprogress.Step {
	return []stepprogress.Step{
		{Name: "Setup", Status: stepprogress.StatusCompleted},
		{Name: "Code Review", Status: stepprogress.StatusCompleted},
		{Name: "Implementation", Status: stepprogress.StatusRunning},
		{Name: "Testing", Status: stepprogress.StatusPending},
	}
}
