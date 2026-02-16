// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

// GetProjectID / GetTaskID methods allow the API server's WebSocket filter
// to match events without maintaining an exhaustive type switch.

func (e TasksLoadedEvent) GetProjectID() string          { return e.ProjectID }
func (e CommitsLoadedEvent) GetProjectID() string         { return e.ProjectID }
func (e TaskCreationStartedEvent) GetProjectID() string   { return e.ProjectID }
func (e TaskLifecycleEvent) GetProjectID() string         { return e.ProjectID }
func (e TaskLifecycleEvent) GetTaskID() string            { return e.TaskID }
func (e AIActivityBatchEvent) GetProjectID() string       { return e.ProjectID }
func (e AIActivityBatchEvent) GetTaskID() string          { return e.TaskID }
func (e AIStreamStartEvent) GetProjectID() string         { return e.ProjectID }
func (e AIStreamStartEvent) GetTaskID() string            { return e.TaskID }
func (e AIStreamEndEvent) GetProjectID() string           { return e.ProjectID }
func (e AIStreamEndEvent) GetTaskID() string              { return e.TaskID }
func (e PipelineRunStartedEvent) GetProjectID() string    { return e.ProjectID }
func (e PipelineRunsLoadedEvent) GetProjectID() string    { return e.ProjectID }
func (e ErrorEvent) GetTaskID() string                    { return e.TaskID }
func (e PipelineLifecycleEvent) GetProjectID() string     { return e.ProjectID }
func (e PipelineLifecycleEvent) GetRunID() string         { return e.RunID }
