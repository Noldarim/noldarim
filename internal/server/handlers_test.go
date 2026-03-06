// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"
)

type stubDataReader struct {
	getPipelineRunFn       func(ctx context.Context, runID string) (*models.PipelineRun, error)
	getAIActivityByRunIDFn func(ctx context.Context, runID string) ([]*models.AIActivityRecord, error)
}

func (s *stubDataReader) LoadProjects(ctx context.Context) (map[string]*models.Project, error) {
	return map[string]*models.Project{}, nil
}

func (s *stubDataReader) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	return &models.Project{}, nil
}

func (s *stubDataReader) LoadTasks(ctx context.Context, projectID string) (map[string]*models.Task, error) {
	return map[string]*models.Task{}, nil
}

func (s *stubDataReader) GetAIActivityByTask(ctx context.Context, taskID string) ([]*models.AIActivityRecord, error) {
	return nil, nil
}

func (s *stubDataReader) GetPipelineRunsByProject(ctx context.Context, projectID string) ([]*models.PipelineRun, error) {
	return nil, nil
}

func (s *stubDataReader) GetPipelineRun(ctx context.Context, runID string) (*models.PipelineRun, error) {
	if s.getPipelineRunFn != nil {
		return s.getPipelineRunFn(ctx, runID)
	}
	return nil, nil
}

func (s *stubDataReader) GetAIActivityByRunID(ctx context.Context, runID string) ([]*models.AIActivityRecord, error) {
	if s.getAIActivityByRunIDFn != nil {
		return s.getAIActivityByRunIDFn(ctx, runID)
	}
	return nil, nil
}

func (s *stubDataReader) GetContainerLogsByRun(ctx context.Context, runID string) ([]*models.ContainerLog, error) {
	return nil, nil
}

type stubPipelineMutator struct {
	startPipelineFn   func(ctx context.Context, params services.StartPipelineParams) (*services.PipelineRunResult, error)
	promotePipelineFn func(ctx context.Context, params services.PromotePipelineParams) (*services.PipelineRunResult, error)
	getMergeQueueFn   func(ctx context.Context, projectID string) (*types.MergeQueueState, error)
}

func (s *stubPipelineMutator) CreateProject(ctx context.Context, name, description, repoPath string) (*models.Project, error) {
	return nil, nil
}

func (s *stubPipelineMutator) CreateTask(ctx context.Context, params services.CreateTaskParams) (*services.PipelineRunResult, error) {
	return nil, nil
}

func (s *stubPipelineMutator) ToggleTask(ctx context.Context, projectID, taskID string) (models.TaskStatus, error) {
	return models.TaskStatusPending, nil
}

func (s *stubPipelineMutator) DeleteTask(ctx context.Context, projectID, taskID string) error {
	return nil
}

func (s *stubPipelineMutator) StartPipeline(ctx context.Context, params services.StartPipelineParams) (*services.PipelineRunResult, error) {
	if s.startPipelineFn != nil {
		return s.startPipelineFn(ctx, params)
	}
	return &services.PipelineRunResult{}, nil
}

func (s *stubPipelineMutator) CancelPipeline(ctx context.Context, runID, reason string) (*services.CancelResult, error) {
	return nil, nil
}

func (s *stubPipelineMutator) PromotePipeline(ctx context.Context, params services.PromotePipelineParams) (*services.PipelineRunResult, error) {
	if s.promotePipelineFn != nil {
		return s.promotePipelineFn(ctx, params)
	}
	return &services.PipelineRunResult{}, nil
}

func (s *stubPipelineMutator) GetMergeQueueState(ctx context.Context, projectID string) (*types.MergeQueueState, error) {
	if s.getMergeQueueFn != nil {
		return s.getMergeQueueFn(ctx, projectID)
	}
	return &types.MergeQueueState{}, nil
}

type stubGitManager struct{}

func (s *stubGitManager) GetService(path string) (*services.GitServiceHandle, error) {
	return nil, nil
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestGetPipelineRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{
						ID:        runID,
						ProjectID: "project-1",
						StepResults: []models.StepResult{
							{ID: "step-result-1", StepID: "step-1"},
						},
					}, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/run-1", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.GetPipelineRun(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var run models.PipelineRun
		if err := json.NewDecoder(rec.Body).Decode(&run); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if run.ID != "run-1" {
			t.Fatalf("expected run ID run-1, got %q", run.ID)
		}
		if len(run.StepResults) != 1 {
			t.Fatalf("expected one step result, got %d", len(run.StepResults))
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return nil, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/run-1", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.GetPipelineRun(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("service error", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return nil, errors.New("db failure")
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/run-1", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.GetPipelineRun(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestGetPipelineRunAIActivity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{ID: runID, ProjectID: "project-123"}, nil
				},
				getAIActivityByRunIDFn: func(ctx context.Context, runID string) ([]*models.AIActivityRecord, error) {
					return []*models.AIActivityRecord{
						{EventID: "evt-1", RunID: runID, TaskID: runID},
					}, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/run-42/activity", nil), "runId", "run-42")
		rec := httptest.NewRecorder()
		h.GetPipelineRunAIActivity(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var payload protocol.AIActivityBatchEvent
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload.ProjectID != "project-123" {
			t.Fatalf("expected project ID project-123, got %q", payload.ProjectID)
		}
		if payload.TaskID != "run-42" {
			t.Fatalf("expected task ID run-42, got %q", payload.TaskID)
		}
		if len(payload.Activities) != 1 {
			t.Fatalf("expected 1 activity, got %d", len(payload.Activities))
		}
	})

	t.Run("activity load error", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{ID: runID, ProjectID: "project-123"}, nil
				},
				getAIActivityByRunIDFn: func(ctx context.Context, runID string) ([]*models.AIActivityRecord, error) {
					return nil, errors.New("load failed")
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/run-42/activity", nil), "runId", "run-42")
		rec := httptest.NewRecorder()
		h.GetPipelineRunAIActivity(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})

	t.Run("run not found", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return nil, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/pipelines/run-42/activity", nil), "runId", "run-42")
		rec := httptest.NewRecorder()
		h.GetPipelineRunAIActivity(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestGetAgentDefaults(t *testing.T) {
	h := NewHandlers(
		nil,
		&stubDataReader{},
		&stubGitManager{},
		&stubPipelineMutator{},
		AgentDefaultsResponse{
			ToolName:    "claude",
			ToolVersion: "4.5",
			FlagFormat:  "space",
			ToolOptions: map[string]any{"model": "claude-sonnet-4-5"},
		},
		"",
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/defaults", nil)
	h.GetAgentDefaults(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload AgentDefaultsResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ToolName != "claude" {
		t.Fatalf("expected tool_name claude, got %q", payload.ToolName)
	}
}

func TestStartPipelineSnakeCaseStepDecoding(t *testing.T) {
	var captured services.StartPipelineParams
	h := NewHandlers(
		nil,
		&stubDataReader{},
		&stubGitManager{},
		&stubPipelineMutator{
			startPipelineFn: func(ctx context.Context, params services.StartPipelineParams) (*services.PipelineRunResult, error) {
				captured = params
				return &services.PipelineRunResult{RunID: "run-xyz", ProjectID: params.ProjectID}, nil
			},
		},
		AgentDefaultsResponse{},
		"",
	)

	body := []byte(`{
		"name":"Pipeline from desktop",
		"steps":[
			{
				"step_id":"analyze",
				"name":"Analyze",
				"agent_config":{
					"tool_name":"claude",
					"prompt_template":"Analyze code",
					"flag_format":"space"
				}
			}
		]
	}`)

	req := withURLParam(
		httptest.NewRequest(http.MethodPost, "/api/v1/projects/project-99/pipelines", bytes.NewReader(body)),
		"id",
		"project-99",
	)
	rec := httptest.NewRecorder()
	h.StartPipeline(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if captured.ProjectID != "project-99" {
		t.Fatalf("expected project ID project-99, got %q", captured.ProjectID)
	}
	if len(captured.Steps) != 1 {
		t.Fatalf("expected one step, got %d", len(captured.Steps))
	}
	if captured.Steps[0].StepID != "analyze" {
		t.Fatalf("expected step_id analyze, got %q", captured.Steps[0].StepID)
	}
	if captured.Steps[0].Name != "Analyze" {
		t.Fatalf("expected step name Analyze, got %q", captured.Steps[0].Name)
	}
	if captured.Steps[0].AgentConfig == nil || captured.Steps[0].AgentConfig.ToolName != "claude" {
		t.Fatalf("expected agent_config.tool_name=claude")
	}
}

func TestPromotePipeline(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{
						ID:        runID,
						ProjectID: "project-1",
						Status:    models.PipelineRunStatusCompleted,
					}, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{
				promotePipelineFn: func(ctx context.Context, params services.PromotePipelineParams) (*services.PipelineRunResult, error) {
					return &services.PipelineRunResult{
						RunID:     params.SourceRunID,
						ProjectID: "project-1",
						Status:    "queued",
					}, nil
				},
			},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/run-1/promote", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.PromotePipeline(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{ID: runID, ProjectID: "project-1", Status: models.PipelineRunStatusCompleted}, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{
				promotePipelineFn: func(ctx context.Context, params services.PromotePipelineParams) (*services.PipelineRunResult, error) {
					return nil, services.ErrRunNotFound
				},
			},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/run-1/promote", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.PromotePipeline(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("not completed (conflict)", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{ID: runID, ProjectID: "project-1", Status: models.PipelineRunStatusRunning}, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{
				promotePipelineFn: func(ctx context.Context, params services.PromotePipelineParams) (*services.PipelineRunResult, error) {
					return nil, services.ErrRunNotCompleted
				},
			},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/run-1/promote", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.PromotePipeline(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cannot promote a promote run (bad request)", func(t *testing.T) {
		h := NewHandlers(
			nil,
			&stubDataReader{
				getPipelineRunFn: func(ctx context.Context, runID string) (*models.PipelineRun, error) {
					return &models.PipelineRun{ID: runID, ProjectID: "project-1", RunType: models.PipelineRunTypePromote, Status: models.PipelineRunStatusCompleted}, nil
				},
			},
			&stubGitManager{},
			&stubPipelineMutator{
				promotePipelineFn: func(ctx context.Context, params services.PromotePipelineParams) (*services.PipelineRunResult, error) {
					return nil, services.ErrCannotPromotePromote
				},
			},
			AgentDefaultsResponse{},
			"",
		)

		req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/pipelines/run-1/promote", nil), "runId", "run-1")
		rec := httptest.NewRecorder()
		h.PromotePipeline(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}
