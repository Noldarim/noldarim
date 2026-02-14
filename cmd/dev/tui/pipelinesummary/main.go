// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/tui/components/pipelinesummary"
)

func main() {
	// Show mock data for demo (comment out to use DB data)
	data := mockData()
	// data := loadSummaryData()
	component := pipelinesummary.New().SetData(data)
	fmt.Println(component.View())
}

func loadSummaryData() pipelinesummary.SummaryData {
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		return mockData()
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return mockData()
	}
	defer dataService.Close()

	ctx := context.Background()
	run, err := dataService.GetLatestPipelineRun(ctx)
	if err != nil || run == nil {
		return mockData()
	}

	return convertPipelineRun(run)
}

func convertPipelineRun(run *models.PipelineRun) pipelinesummary.SummaryData {
	data := pipelinesummary.SummaryData{
		Status:        convertStatus(run.Status),
		TotalSteps:    len(run.StepResults),
		BranchName:    run.BranchName,
		BaseCommitSHA: run.BaseCommitSHA,
		HeadCommitSHA: run.HeadCommitSHA,
		ErrorMessage:  run.ErrorMessage,
	}

	// Calculate duration
	if run.StartedAt != nil {
		if run.CompletedAt != nil {
			data.Duration = run.CompletedAt.Sub(*run.StartedAt)
		} else {
			data.Duration = time.Since(*run.StartedAt)
		}
	}

	// Aggregate step data
	for _, step := range run.StepResults {
		if step.Status == models.StepStatusCompleted {
			data.CompletedSteps++
		} else if step.Status == models.StepStatusFailed {
			data.FailedSteps++
		}

		data.TotalTokens += step.InputTokens + step.OutputTokens
		data.CacheHitTokens += step.CacheReadTokens
		data.FilesChanged += step.FilesChanged
		data.Insertions += step.Insertions
		data.Deletions += step.Deletions
	}

	return data
}

func convertStatus(s models.PipelineRunStatus) pipelinesummary.Status {
	switch s {
	case models.PipelineRunStatusPending:
		return pipelinesummary.StatusPending
	case models.PipelineRunStatusRunning:
		return pipelinesummary.StatusRunning
	case models.PipelineRunStatusCompleted:
		return pipelinesummary.StatusCompleted
	case models.PipelineRunStatusFailed:
		return pipelinesummary.StatusFailed
	default:
		return pipelinesummary.StatusPending
	}
}

func mockData() pipelinesummary.SummaryData {
	return pipelinesummary.SummaryData{
		Status:         pipelinesummary.StatusCompleted,
		Duration:       3*time.Minute + 42*time.Second,
		TotalSteps:     4,
		CompletedSteps: 4,
		TotalTokens:    53350,
		CacheHitTokens: 12340,
		FilesChanged:   7,
		Insertions:     234,
		Deletions:      45,
		BranchName:     "feature/add-auth",
		BaseCommitSHA:  "abc1234def5678",
		HeadCommitSHA:  "fed8765cba4321",
	}
}
