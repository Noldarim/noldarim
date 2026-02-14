// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"fmt"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/tui/components/activityfeed"
)

func main() {
	activities := loadActivities()
	component := activityfeed.New().SetActivities(activities).SetMaxItems(10)
	fmt.Println(component.View())
}

func loadActivities() []activityfeed.Activity {
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		return mockActivities()
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return mockActivities()
	}
	defer dataService.Close()

	ctx := context.Background()

	// Get latest task to find activities
	task, err := dataService.GetLatestTask(ctx)
	if err != nil || task == nil {
		return mockActivities()
	}

	taskID := task.ID
	records, err := dataService.GetAIActivityByTask(ctx, taskID)
	if err != nil || len(records) == 0 {
		return mockActivities()
	}

	activities := make([]activityfeed.Activity, len(records))
	for i, r := range records {
		activities[i] = activityfeed.Activity{
			EventType:      convertEventType(r.EventType),
			ToolName:       r.ToolName,
			ContentPreview: r.ContentPreview,
			FilePath:       r.FilePath,
			ToolSuccess:    r.ToolSuccess,
			ToolError:      r.ToolError,
		}
	}

	return activities
}

func convertEventType(t models.AIEventType) activityfeed.EventType {
	switch t {
	case models.AIEventToolUse:
		return activityfeed.EventToolUse
	case models.AIEventToolResult:
		return activityfeed.EventToolResult
	case models.AIEventThinking:
		return activityfeed.EventThinking
	case models.AIEventAIOutput:
		return activityfeed.EventAIOutput
	case models.AIEventSubagentStart:
		return activityfeed.EventSubagentStart
	case models.AIEventSubagentStop:
		return activityfeed.EventSubagentStop
	case models.AIEventError:
		return activityfeed.EventError
	default:
		return activityfeed.EventType(t)
	}
}

func mockActivities() []activityfeed.Activity {
	yes := true
	no := false
	return []activityfeed.Activity{
		{EventType: activityfeed.EventThinking, ContentPreview: "Analyzing the codebase structure..."},
		{EventType: activityfeed.EventToolUse, ToolName: "Read", FilePath: "internal/config/config.go"},
		{EventType: activityfeed.EventToolResult, ToolSuccess: &yes, ContentPreview: "Read 245 lines"},
		{EventType: activityfeed.EventToolUse, ToolName: "Grep", ContentPreview: "searching for 'func New'"},
		{EventType: activityfeed.EventToolResult, ToolSuccess: &yes, ContentPreview: "Found 12 matches"},
		{EventType: activityfeed.EventThinking, ContentPreview: "The config system uses YAML parsing..."},
		{EventType: activityfeed.EventToolUse, ToolName: "Edit", FilePath: "internal/tui/main.go"},
		{EventType: activityfeed.EventToolResult, ToolSuccess: &no, ToolError: "File not found"},
		{EventType: activityfeed.EventAIOutput, ContentPreview: "I've updated the configuration handler to support the new format."},
	}
}
