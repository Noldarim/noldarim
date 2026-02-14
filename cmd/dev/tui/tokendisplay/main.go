// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/tui/components/tokendisplay"
)

func main() {
	tokenData := loadTokenData()
	component := tokendisplay.New().SetData(tokenData)
	fmt.Println(component.View())
}

func loadTokenData() tokendisplay.TokenData {
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

	var data tokendisplay.TokenData
	for _, step := range run.StepResults {
		data.InputTokens += step.InputTokens
		data.OutputTokens += step.OutputTokens
		data.CacheReadTokens += step.CacheReadTokens
		data.CacheCreateTokens += step.CacheCreateTokens
	}

	if data.InputTokens == 0 && data.OutputTokens == 0 {
		return mockData()
	}

	return data
}

func mockData() tokendisplay.TokenData {
	return tokendisplay.TokenData{
		InputTokens:       45230,
		OutputTokens:      8120,
		CacheReadTokens:   12340,
		CacheCreateTokens: 5200,
	}
}
