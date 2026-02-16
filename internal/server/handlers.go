// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/go-chi/chi/v5"
)

type dataReader interface {
	LoadProjects(ctx context.Context) (map[string]*models.Project, error)
	GetProject(ctx context.Context, projectID string) (*models.Project, error)
	LoadTasks(ctx context.Context, projectID string) (map[string]*models.Task, error)
	GetAIActivityByTask(ctx context.Context, taskID string) ([]*models.AIActivityRecord, error)
	GetPipelineRunsByProject(ctx context.Context, projectID string) ([]*models.PipelineRun, error)
	GetPipelineRun(ctx context.Context, runID string) (*models.PipelineRun, error)
	GetAIActivityByRunID(ctx context.Context, runID string) ([]*models.AIActivityRecord, error)
}

type gitManager interface {
	GetService(path string) (*services.GitServiceHandle, error)
}

type pipelineMutator interface {
	CreateProject(ctx context.Context, name, description, repoPath string) (*models.Project, error)
	CreateTask(ctx context.Context, params services.CreateTaskParams) (*services.PipelineRunResult, error)
	ToggleTask(ctx context.Context, projectID, taskID string) (models.TaskStatus, error)
	DeleteTask(ctx context.Context, projectID, taskID string) error
	StartPipeline(ctx context.Context, params services.StartPipelineParams) (*services.PipelineRunResult, error)
	CancelPipeline(ctx context.Context, runID, reason string) (*services.CancelResult, error)
}

// AgentDefaultsResponse provides server-side default agent configuration for desktop clients.
type AgentDefaultsResponse struct {
	ToolName    string                 `json:"tool_name"`
	ToolVersion string                 `json:"tool_version"`
	FlagFormat  string                 `json:"flag_format"`
	ToolOptions map[string]any `json:"tool_options"`
}

// Handlers holds dependencies for HTTP handlers.
type Handlers struct {
	broadcaster *EventBroadcaster
	data        dataReader
	git         gitManager
	pipeline    pipelineMutator
	defaults    AgentDefaultsResponse
}

// NewHandlers creates the handler set.
func NewHandlers(
	broadcaster *EventBroadcaster,
	data dataReader,
	git gitManager,
	pipeline pipelineMutator,
	defaults AgentDefaultsResponse,
) *Handlers {
	return &Handlers{
		broadcaster: broadcaster,
		data:        data,
		git:         git,
		pipeline:    pipeline,
		defaults:    defaults,
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		getLog().Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func writeError(w http.ResponseWriter, status int, clientMsg string, err error) {
	if err != nil {
		getLog().Error().Err(err).Msg(clientMsg)
	}
	writeJSON(w, status, map[string]string{"error": clientMsg})
}

// --- GET handlers (direct reads, no command channel) ---

// GetProjects handles GET /api/v1/projects
func (h *Handlers) GetProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.data.LoadProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load projects", err)
		return
	}
	writeJSON(w, http.StatusOK, protocol.ProjectsLoadedEvent{Projects: projects})
}

// GetTasks handles GET /api/v1/projects/{id}/tasks
func (h *Handlers) GetTasks(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	ctx := r.Context()

	project, err := h.data.GetProject(ctx, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load project", err)
		return
	}

	tasks, err := h.data.LoadTasks(ctx, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load tasks", err)
		return
	}

	writeJSON(w, http.StatusOK, protocol.TasksLoadedEvent{
		ProjectID:      projectID,
		ProjectName:    project.Name,
		RepositoryPath: project.RepositoryPath,
		Tasks:          tasks,
	})
}

// GetCommits handles GET /api/v1/projects/{id}/commits
func (h *Handlers) GetCommits(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	ctx := r.Context()
	const maxLimit = 500
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	project, err := h.data.GetProject(ctx, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load project", err)
		return
	}

	if project.RepositoryPath == "" {
		writeJSON(w, http.StatusOK, protocol.CommitsLoadedEvent{
			ProjectID:      projectID,
			RepositoryPath: "",
			Commits:        []protocol.CommitInfo{},
		})
		return
	}

	gitHandle, err := h.git.GetService(project.RepositoryPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to access git repository", err)
		return
	}
	defer gitHandle.Release()

	commits, err := gitHandle.GetGitService().GetCommitHistory(ctx, project.RepositoryPath, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load commits", err)
		return
	}

	commitInfos := make([]protocol.CommitInfo, len(commits))
	for i, c := range commits {
		commitInfos[i] = protocol.CommitInfo{
			Hash:    c.Hash,
			Message: c.Message,
			Author:  c.Author,
			Parents: c.Parents,
		}
	}

	writeJSON(w, http.StatusOK, protocol.CommitsLoadedEvent{
		ProjectID:      projectID,
		RepositoryPath: project.RepositoryPath,
		Commits:        commitInfos,
	})
}

// GetAIActivity handles GET /api/v1/projects/{id}/tasks/{taskId}/activity
func (h *Handlers) GetAIActivity(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	activities, err := h.data.GetAIActivityByTask(r.Context(), taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load AI activity", err)
		return
	}

	writeJSON(w, http.StatusOK, protocol.AIActivityBatchEvent{
		TaskID:     taskID,
		ProjectID:  projectID,
		Activities: activities,
	})
}

// GetPipelineRuns handles GET /api/v1/projects/{id}/pipelines
func (h *Handlers) GetPipelineRuns(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	ctx := r.Context()

	project, err := h.data.GetProject(ctx, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load project", err)
		return
	}

	runs, err := h.data.GetPipelineRunsByProject(ctx, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load pipeline runs", err)
		return
	}

	runsMap := make(map[string]*models.PipelineRun, len(runs))
	for _, run := range runs {
		runsMap[run.ID] = run
	}

	writeJSON(w, http.StatusOK, protocol.PipelineRunsLoadedEvent{
		ProjectID:      projectID,
		ProjectName:    project.Name,
		RepositoryPath: project.RepositoryPath,
		Runs:           runsMap,
	})
}

// GetPipelineRun handles GET /api/v1/pipelines/{runId}
func (h *Handlers) GetPipelineRun(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(chi.URLParam(r, "runId"))
	if runID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "runId is required"})
		return
	}
	run, err := h.data.GetPipelineRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load pipeline run", err)
		return
	}
	if run == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pipeline run not found"})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// GetPipelineRunAIActivity handles GET /api/v1/pipelines/{runId}/activity
func (h *Handlers) GetPipelineRunAIActivity(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(chi.URLParam(r, "runId"))
	if runID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "runId is required"})
		return
	}
	run, err := h.data.GetPipelineRun(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load pipeline run", err)
		return
	}
	if run == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pipeline run not found"})
		return
	}

	activities, err := h.data.GetAIActivityByRunID(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load AI activity", err)
		return
	}

	writeJSON(w, http.StatusOK, protocol.AIActivityBatchEvent{
		TaskID:     runID,
		ProjectID:  run.ProjectID,
		Activities: activities,
	})
}

// GetAgentDefaults handles GET /api/v1/agent/defaults
func (h *Handlers) GetAgentDefaults(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.defaults)
}

// --- POST/PUT/DELETE handlers (direct service calls) ---

// createProjectRequest is the JSON body for project creation.
type createProjectRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	RepositoryPath string `json:"repository_path"`
}

// CreateProject handles POST /api/v1/projects
func (h *Handlers) CreateProject(w http.ResponseWriter, r *http.Request) {
	var body createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.RepositoryPath = strings.TrimSpace(body.RepositoryPath)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if body.RepositoryPath == "" || !filepath.IsAbs(body.RepositoryPath) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repository_path must be an absolute path"})
		return
	}
	if filepath.Clean(body.RepositoryPath) != body.RepositoryPath {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repository_path contains invalid path components"})
		return
	}

	project, err := h.pipeline.CreateProject(r.Context(), body.Name, body.Description, body.RepositoryPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create project", err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

// createTaskRequest is the JSON body for task creation.
type createTaskRequest struct {
	Title         string                     `json:"title"`
	Description   string                     `json:"description"`
	BaseCommitSHA string                     `json:"base_commit_sha,omitempty"`
	AgentConfig   *protocol.AgentConfigInput `json:"agent_config,omitempty"`
}

// CreateTask handles POST /api/v1/projects/{id}/tasks
func (h *Handlers) CreateTask(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	var body createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	result, err := h.pipeline.CreateTask(r.Context(), services.CreateTaskParams{
		ProjectID:     projectID,
		Title:         body.Title,
		Description:   body.Description,
		BaseCommitSHA: body.BaseCommitSHA,
		AgentConfig:   body.AgentConfig,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create task", err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// ToggleTask handles POST /api/v1/projects/{id}/tasks/{taskId}/toggle
func (h *Handlers) ToggleTask(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	newStatus, err := h.pipeline.ToggleTask(r.Context(), projectID, taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to toggle task", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": newStatus.String()})
}

// DeleteTask handles DELETE /api/v1/projects/{id}/tasks/{taskId}
func (h *Handlers) DeleteTask(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := h.pipeline.DeleteTask(r.Context(), projectID, taskID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete task", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// startPipelineRequest is the JSON body for pipeline creation.
type startPipelineRequest struct {
	Name            string                     `json:"name"`
	Steps           []startPipelineStepRequest `json:"steps"`
	BaseCommitSHA   string                     `json:"base_commit_sha,omitempty"`
	ForkFromRunID   string                     `json:"fork_from_run_id,omitempty"`
	ForkAfterStepID string                     `json:"fork_after_step_id,omitempty"`
	NoAutoFork      bool                       `json:"no_auto_fork,omitempty"`
}

type startPipelineStepRequest struct {
	StepID      string                     `json:"step_id"`
	Name        string                     `json:"name"`
	AgentConfig *protocol.AgentConfigInput `json:"agent_config,omitempty"`
}

// StartPipeline handles POST /api/v1/projects/{id}/pipelines
func (h *Handlers) StartPipeline(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	var body startPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON body"})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if len(body.Steps) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "steps is required and must not be empty"})
		return
	}

	steps := make([]protocol.StepInput, len(body.Steps))
	for i, step := range body.Steps {
		stepID := strings.TrimSpace(step.StepID)
		stepName := strings.TrimSpace(step.Name)
		if stepID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "steps[].step_id is required"})
			return
		}
		if stepName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "steps[].name is required"})
			return
		}
		steps[i] = protocol.StepInput{
			StepID:      stepID,
			Name:        stepName,
			AgentConfig: step.AgentConfig,
		}
	}

	result, err := h.pipeline.StartPipeline(r.Context(), services.StartPipelineParams{
		ProjectID:       projectID,
		Name:            body.Name,
		Steps:           steps,
		BaseCommitSHA:   body.BaseCommitSHA,
		ForkFromRunID:   body.ForkFromRunID,
		ForkAfterStepID: body.ForkAfterStepID,
		NoAutoFork:      body.NoAutoFork,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start pipeline", err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// cancelPipelineRequest is the JSON body for pipeline cancellation.
type cancelPipelineRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CancelPipeline handles POST /api/v1/pipelines/{runId}/cancel
func (h *Handlers) CancelPipeline(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "runId")
	var body cancelPipelineRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
	}

	result, err := h.pipeline.CancelPipeline(r.Context(), runID, body.Reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to cancel pipeline", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
