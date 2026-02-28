// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workflows"
	"github.com/noldarim/noldarim/internal/protocol"
)

var (
	pipelineLog     *zerolog.Logger
	pipelineLogOnce sync.Once

	// Sentinel errors for promote validation
	ErrRunNotFound          = errors.New("source run not found")
	ErrRunNotCompleted      = errors.New("source run is not completed")
	ErrCannotPromotePromote = errors.New("cannot promote a promote run")
)

func getPipelineLog() *zerolog.Logger {
	pipelineLogOnce.Do(func() {
		l := logger.GetOrchestratorLogger().With().Str("component", "pipeline_service").Logger()
		pipelineLog = &l
	})
	return pipelineLog
}

// PipelineService encapsulates business logic for project/task/pipeline mutations.
// Both the TUI orchestrator and the API server call these methods directly.
type PipelineService struct {
	data     *DataService
	git      *GitServiceManager
	temporal TemporalClient
	config   *config.AppConfig
}

// NewPipelineService creates a PipelineService with its dependencies.
func NewPipelineService(data *DataService, git *GitServiceManager, temporal TemporalClient, cfg *config.AppConfig) *PipelineService {
	return &PipelineService{
		data:     data,
		git:      git,
		temporal: temporal,
		config:   cfg,
	}
}

// --- Result / param types ---

// PipelineRunResult is the outcome of CreateTask or StartPipeline.
type PipelineRunResult struct {
	RunID           string
	ProjectID       string
	Name            string
	WorkflowID      string
	AlreadyExists   bool
	Status          string // protocol.PipelineStatus* constant
	ForkFromRunID   string
	ForkAfterStepID string
	SkippedSteps    int
}

// CancelResult is the outcome of CancelPipeline.
type CancelResult struct {
	RunID          string
	Reason         string
	WorkflowStatus string
}

// CreateTaskParams groups input for CreateTask.
type CreateTaskParams struct {
	ProjectID     string
	Title         string
	Description   string
	BaseCommitSHA string
	AgentConfig   *protocol.AgentConfigInput
}

// StartPipelineParams groups input for StartPipeline.
type StartPipelineParams struct {
	ProjectID       string
	Name            string
	Steps           []protocol.StepInput
	BaseCommitSHA   string
	ForkFromRunID   string
	ForkAfterStepID string
	NoAutoFork      bool
	AutoPromote     bool
}

// --- Public methods ---

// CreateProject validates inputs, initialises a git service, and persists a new project.
func (ps *PipelineService) CreateProject(ctx context.Context, name, description, repoPath string) (*models.Project, error) {
	if err := validateProjectInputs(name, description, repoPath); err != nil {
		return nil, err
	}

	// Initialise git service (may auto-create repo)
	gitHandle, err := ps.git.GetService(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git service: %w", err)
	}
	defer gitHandle.Release()

	project, err := ps.data.CreateProject(ctx, name, description, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	getPipelineLog().Info().Str("project_id", project.ID).Str("name", project.Name).Msg("Created project")
	return project, nil
}

// ToggleTask toggles a task's completion status and returns the new status.
// Completed → Pending, Pending/InProgress → Completed.
func (ps *PipelineService) ToggleTask(ctx context.Context, projectID, taskID string) (models.TaskStatus, error) {
	task, err := ps.data.GetTask(ctx, taskID)
	if err != nil {
		return 0, fmt.Errorf("failed to get task: %w", err)
	}
	newStatus := models.TaskStatusCompleted
	if task.Status == models.TaskStatusCompleted {
		newStatus = models.TaskStatusPending
	}
	if err := ps.data.UpdateTaskStatus(ctx, taskID, newStatus); err != nil {
		return 0, fmt.Errorf("failed to update task status: %w", err)
	}
	getPipelineLog().Info().Str("project_id", projectID).Str("task_id", taskID).Str("new_status", newStatus.String()).Msg("Task toggled")
	return newStatus, nil
}

// DeleteTask deletes a task by ID.
func (ps *PipelineService) DeleteTask(ctx context.Context, projectID, taskID string) error {
	if err := ps.data.DeleteTask(ctx, taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	getPipelineLog().Info().Str("project_id", projectID).Str("task_id", taskID).Msg("Deleted task")
	return nil
}

// CreateTask creates a single-step pipeline run (a "task" is just a 1-step pipeline).
func (ps *PipelineService) CreateTask(ctx context.Context, params CreateTaskParams) (*PipelineRunResult, error) {
	repoPath, err := ps.data.GetProjectRepositoryPath(ctx, params.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("could not get repository path for project: %w", err)
	}

	baseCommitSHA := params.BaseCommitSHA
	if baseCommitSHA == "" {
		baseCommitSHA, err = ps.resolveCurrentCommit(ctx, repoPath)
		if err != nil {
			return nil, err
		}
	}

	// Build agent config for the single step
	stepAgentConfig := ps.buildStepAgentConfig(params.AgentConfig, params.Title, params.Description)

	step := models.StepDefinition{
		StepID:      "main",
		Name:        params.Title,
		Description: params.Description,
		AgentConfig: stepAgentConfig,
	}
	steps := []models.StepDefinition{step}

	runID := ComputeRunID(baseCommitSHA, workflows.PipelineWorkflowVersion, steps)
	workflowID := fmt.Sprintf("%s-pipeline", runID)

	// Check idempotency
	if result, done := ps.checkIdempotency(ctx, workflowID, runID, params.ProjectID, params.Title); done {
		return result, nil
	}

	input := ps.buildWorkflowInput(runID, params.ProjectID, params.Title, steps, repoPath, baseCommitSHA, "", "", false)

	if _, err := ps.temporal.StartWorkflow(ctx, workflowID, workflows.PipelineWorkflowName, input); err != nil {
		return nil, fmt.Errorf("failed to start pipeline workflow: %w", err)
	}

	getPipelineLog().Info().
		Str("project_id", params.ProjectID).Str("run_id", runID).Str("workflow_id", workflowID).
		Msg("Created task as single-step pipeline")

	return &PipelineRunResult{
		RunID:      runID,
		ProjectID:  params.ProjectID,
		Name:       params.Title,
		WorkflowID: workflowID,
	}, nil
}

// StartPipeline starts a multi-step pipeline workflow.
func (ps *PipelineService) StartPipeline(ctx context.Context, params StartPipelineParams) (*PipelineRunResult, error) {
	repoPath, err := ps.data.GetProjectRepositoryPath(ctx, params.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("could not get repository path for project: %w", err)
	}

	baseCommitSHA := params.BaseCommitSHA
	if baseCommitSHA == "" {
		baseCommitSHA, err = ps.resolveCurrentCommit(ctx, repoPath)
		if err != nil {
			return nil, err
		}
	}

	// Convert protocol steps to model steps
	modelSteps := convertProtocolSteps(params.Steps)

	// Determine fork parameters
	forkFromRunID, forkAfterStepID, skippedSteps := ps.resolveForkParams(ctx, params, modelSteps, baseCommitSHA)

	runID := ComputeRunID(baseCommitSHA, workflows.PipelineWorkflowVersion, modelSteps)
	workflowID := fmt.Sprintf("%s-pipeline", runID)

	// Check idempotency
	if result, done := ps.checkIdempotency(ctx, workflowID, runID, params.ProjectID, params.Name); done {
		return result, nil
	}

	input := ps.buildWorkflowInput(runID, params.ProjectID, params.Name, modelSteps, repoPath, baseCommitSHA, forkFromRunID, forkAfterStepID, params.AutoPromote)

	if _, err := ps.temporal.StartWorkflow(ctx, workflowID, workflows.PipelineWorkflowName, input); err != nil {
		return nil, fmt.Errorf("failed to start pipeline workflow: %w", err)
	}

	getPipelineLog().Info().
		Str("project_id", params.ProjectID).Str("run_id", runID).Str("workflow_id", workflowID).
		Str("fork_from", forkFromRunID).Str("fork_after", forkAfterStepID).Int("skipped_steps", skippedSteps).
		Bool("auto_promote", params.AutoPromote).
		Msg("Started pipeline workflow")

	return &PipelineRunResult{
		RunID:           runID,
		ProjectID:       params.ProjectID,
		Name:            params.Name,
		WorkflowID:      workflowID,
		ForkFromRunID:   forkFromRunID,
		ForkAfterStepID: forkAfterStepID,
		SkippedSteps:    skippedSteps,
	}, nil
}

// CancelPipeline cancels a running pipeline and waits for termination.
// The caller's ctx is intentionally unused — cancellation uses a detached context
// so it completes even if the caller's context is already cancelled.
func (ps *PipelineService) CancelPipeline(_ context.Context, runID, reason string) (*CancelResult, error) {
	workflowID := fmt.Sprintf("%s-pipeline", runID)
	if reason == "" {
		reason = "User requested cancellation"
	}

	getPipelineLog().Info().Str("run_id", runID).Str("workflow_id", workflowID).Str("reason", reason).Msg("Cancelling pipeline")

	// Use a detached context — cancellation must complete even if caller's context is cancelled
	cancelCtx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()

	if err := ps.temporal.CancelWorkflow(cancelCtx, workflowID); err != nil {
		return nil, fmt.Errorf("failed to cancel workflow: %w", err)
	}

	finalStatus := ps.waitForWorkflowTermination(cancelCtx, workflowID)

	getPipelineLog().Info().Str("run_id", runID).Str("final_status", finalStatus).Msg("Pipeline cancellation completed")

	return &CancelResult{
		RunID:          runID,
		Reason:         reason,
		WorkflowStatus: finalStatus,
	}, nil
}

// --- Private helpers ---

// truncateID safely truncates an ID string to at most n characters.
func truncateID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}

func (ps *PipelineService) mainBranch() string {
	if b := ps.config.Git.DefaultBranch; b != "" {
		return b
	}
	return "main"
}

func validateProjectInputs(name, description, repoPath string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	if len(name) > 255 {
		return fmt.Errorf("project name must be 255 characters or less")
	}
	if len(description) > 1000 {
		return fmt.Errorf("project description must be 1000 characters or less")
	}
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}
	if !filepath.IsAbs(repoPath) {
		return fmt.Errorf("repository path must be absolute")
	}
	return nil
}

func (ps *PipelineService) resolveCurrentCommit(ctx context.Context, repoPath string) (string, error) {
	if ps.git == nil {
		return "", fmt.Errorf("git service manager not configured")
	}
	gitHandle, err := ps.git.GetService(repoPath)
	if err != nil {
		return "", fmt.Errorf("could not access git repository: %w", err)
	}
	sha, err := gitHandle.GetGitService().GetCurrentCommit(ctx, repoPath)
	gitHandle.Release()
	if err != nil {
		return "", fmt.Errorf("could not get current commit: %w", err)
	}
	return sha, nil
}

func (ps *PipelineService) buildStepAgentConfig(protocolCfg *protocol.AgentConfigInput, title, description string) *models.StepAgentConfig {
	if protocolCfg != nil {
		return &models.StepAgentConfig{
			ToolName:       protocolCfg.ToolName,
			ToolVersion:    protocolCfg.ToolVersion,
			PromptTemplate: protocolCfg.PromptTemplate,
			Variables:      protocolCfg.Variables,
			ToolOptions:    protocolCfg.ToolOptions,
			FlagFormat:     protocolCfg.FlagFormat,
		}
	}
	if ps.config.Agent.DefaultTool == "" {
		return nil
	}
	agentVariables := make(map[string]string)
	for k := range ps.config.Agent.Variables {
		switch k {
		case "title":
			agentVariables[k] = title
		case "description":
			agentVariables[k] = description
		default:
			agentVariables[k] = ps.config.Agent.Variables[k]
		}
	}
	return &models.StepAgentConfig{
		ToolName:       ps.config.Agent.DefaultTool,
		ToolVersion:    ps.config.Agent.DefaultVersion,
		PromptTemplate: ps.config.Agent.PromptTemplate,
		Variables:      agentVariables,
		ToolOptions:    ps.config.Agent.ToolOptions,
		FlagFormat:     ps.config.Agent.FlagFormat,
	}
}

// checkIdempotency checks if a workflow already exists and returns a result if so.
// Returns (result, true) if the caller should return early, or (nil, false) to continue.
func (ps *PipelineService) checkIdempotency(ctx context.Context, workflowID, runID, projectID, name string) (*PipelineRunResult, bool) {
	status, err := ps.temporal.GetWorkflowStatus(ctx, workflowID)
	if err != nil {
		return nil, false // not found → continue
	}

	switch status {
	case temporal.WorkflowStatusRunning:
		return &PipelineRunResult{
			RunID:         runID,
			ProjectID:     projectID,
			Name:          name,
			AlreadyExists: true,
			Status:        string(protocol.PipelineStatusRunning),
		}, true

	case temporal.WorkflowStatusCompleted:
		return &PipelineRunResult{
			RunID:         runID,
			ProjectID:     projectID,
			Name:          name,
			AlreadyExists: true,
			Status:        string(protocol.PipelineStatusCompleted),
		}, true

	case temporal.WorkflowStatusFailed,
		temporal.WorkflowStatusCanceled,
		temporal.WorkflowStatusTerminated,
		temporal.WorkflowStatusTimedOut:
		// Continue to retry
		return nil, false

	default:
		return nil, false
	}
}

func (ps *PipelineService) buildWorkflowInput(
	runID, projectID, name string,
	steps []models.StepDefinition,
	repoPath, baseCommitSHA, forkFromRunID, forkAfterStepID string,
	autoPromote bool,
) types.PipelineWorkflowInput {
	promptPrefix := ""
	promptSuffix := ""
	if shouldApplyPromptComposition(steps) {
		promptPrefix = ps.config.Pipeline.PromptPrefix
		promptSuffix = ps.config.Pipeline.PromptSuffix
	}

	input := types.PipelineWorkflowInput{
		RunID:                 runID,
		ProjectID:             projectID,
		Name:                  name,
		Steps:                 steps,
		PromptPrefix:          promptPrefix,
		PromptSuffix:          promptSuffix,
		RepositoryPath:        repoPath,
		BaseCommitSHA:         baseCommitSHA,
		ForkFromRunID:         forkFromRunID,
		ForkAfterStepID:       forkAfterStepID,
		ClaudeConfigPath:      ps.config.Claude.ClaudeJSONHostPath,
		WorkspaceDir:          ps.config.Container.WorkspaceDir,
		OrchestratorTaskQueue: ps.config.Temporal.TaskQueue,
		AutoPromote:           autoPromote,
	}
	if autoPromote {
		input.MainBranch = ps.mainBranch()
	}
	return input
}

func convertProtocolSteps(steps []protocol.StepInput) []models.StepDefinition {
	modelSteps := make([]models.StepDefinition, len(steps))
	for i, step := range steps {
		var agentConfig *models.StepAgentConfig
		if step.AgentConfig != nil {
			agentConfig = &models.StepAgentConfig{
				ToolName:       step.AgentConfig.ToolName,
				ToolVersion:    step.AgentConfig.ToolVersion,
				PromptTemplate: step.AgentConfig.PromptTemplate,
				Variables:      step.AgentConfig.Variables,
				ToolOptions:    step.AgentConfig.ToolOptions,
			}
		}
		modelSteps[i] = models.StepDefinition{
			StepID:      step.StepID,
			Name:        step.Name,
			AgentConfig: agentConfig,
		}
	}
	return modelSteps
}

func (ps *PipelineService) resolveForkParams(
	ctx context.Context,
	params StartPipelineParams,
	modelSteps []models.StepDefinition,
	baseCommitSHA string,
) (forkFromRunID, forkAfterStepID string, skippedSteps int) {
	if params.ForkFromRunID != "" {
		return params.ForkFromRunID, params.ForkAfterStepID, 0
	}
	if params.NoAutoFork {
		return "", "", 0
	}

	autoFork, err := ps.detectAutoFork(ctx, params.ProjectID, modelSteps, baseCommitSHA)
	if err != nil {
		getPipelineLog().Warn().Err(err).Msg("Auto-fork detection failed, proceeding without fork")
		return "", "", 0
	}
	if autoFork.ShouldFork {
		getPipelineLog().Info().
			Str("fork_from", autoFork.ForkFromRunID).Str("fork_after", autoFork.ForkAfterStepID).
			Int("skipped_steps", autoFork.SkippedSteps).Str("reason", autoFork.Reason).
			Msg("Auto-fork detected")
		return autoFork.ForkFromRunID, autoFork.ForkAfterStepID, autoFork.SkippedSteps
	}
	return "", "", 0
}

// AutoForkResult contains the result of auto-fork detection.
type AutoForkResult struct {
	ShouldFork      bool
	ForkFromRunID   string
	ForkAfterStepID string
	SkippedSteps    int
	Reason          string
}

func (ps *PipelineService) detectAutoFork(
	ctx context.Context,
	projectID string,
	steps []models.StepDefinition,
	baseCommitSHA string,
) (*AutoForkResult, error) {
	result := &AutoForkResult{ShouldFork: false}

	if len(steps) < 2 {
		result.Reason = "Pipeline has fewer than 2 steps"
		return result, nil
	}

	runs, err := ps.data.GetRecentSuccessfulRunsWithSteps(ctx, projectID, baseCommitSHA, 10)
	if err != nil {
		return result, fmt.Errorf("failed to query recent runs: %w", err)
	}

	if len(runs) == 0 {
		result.Reason = "No previous successful runs found with same base commit"
		return result, nil
	}

	currentHashes := make([]string, len(steps))
	for i, step := range steps {
		currentHashes[i] = models.ComputeStepDefinitionHash(step)
	}

	var bestRun *models.PipelineRun
	var bestMatchCount int

	for _, run := range runs {
		if len(run.StepResults) == 0 {
			continue
		}
		matchCount := 0
		for i, stepResult := range run.StepResults {
			if i >= len(currentHashes) {
				break
			}
			if stepResult.Status != models.StepStatusCompleted {
				break
			}
			if stepResult.DefinitionHash != currentHashes[i] {
				break
			}
			matchCount++
		}
		if matchCount > bestMatchCount && matchCount < len(steps) {
			bestMatchCount = matchCount
			bestRun = run
		}
	}

	if bestRun == nil || bestMatchCount == 0 {
		result.Reason = "No matching step prefix found in recent runs"
		return result, nil
	}

	forkAfterStepID := bestRun.StepResults[bestMatchCount-1].StepID

	result.ShouldFork = true
	result.ForkFromRunID = bestRun.ID
	result.ForkAfterStepID = forkAfterStepID
	result.SkippedSteps = bestMatchCount
	result.Reason = fmt.Sprintf("Found %d matching steps from run %s", bestMatchCount, truncateID(bestRun.ID, 8))

	return result, nil
}

func (ps *PipelineService) waitForWorkflowTermination(ctx context.Context, workflowID string) string {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "timeout"
		case <-ticker.C:
			status, err := ps.temporal.GetWorkflowStatus(ctx, workflowID)
			if err != nil {
				return "unknown"
			}
			switch status {
			case temporal.WorkflowStatusRunning:
				continue
			case temporal.WorkflowStatusCanceled:
				return "canceled"
			case temporal.WorkflowStatusTerminated:
				return "terminated"
			case temporal.WorkflowStatusFailed:
				return "failed"
			case temporal.WorkflowStatusCompleted:
				return "completed"
			case temporal.WorkflowStatusTimedOut:
				return "timed_out"
			default:
				return "unknown"
			}
		}
	}
}

// ComputeRunID generates a content-based run ID for idempotent pipeline starts.
// Deterministic based on: commit + workflow version + pipeline definition.
func ComputeRunID(baseCommitSHA, workflowVersion string, steps []models.StepDefinition) string {
	h := sha256.New()
	h.Write([]byte(baseCommitSHA))
	h.Write([]byte(workflowVersion))
	for _, step := range steps {
		h.Write([]byte(models.ComputeStepDefinitionHash(step)))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// --- Promote / Merge Queue methods ---

// PromotePipelineParams groups input for PromotePipeline.
type PromotePipelineParams struct {
	SourceRunID string
}

// PromotePipeline queues a completed pipeline run for merge into the project's main branch.
func (ps *PipelineService) PromotePipeline(ctx context.Context, params PromotePipelineParams) (*PipelineRunResult, error) {
	// Validate: source run exists and is completed
	run, err := ps.data.GetPipelineRun(ctx, params.SourceRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("%w: %s", ErrRunNotFound, params.SourceRunID)
	}
	if run.Status != models.PipelineRunStatusCompleted {
		return nil, fmt.Errorf("%w (status=%s)", ErrRunNotCompleted, run.Status.String())
	}
	if run.RunType == models.PipelineRunTypePromote {
		return nil, fmt.Errorf("%w: %s", ErrCannotPromotePromote, params.SourceRunID)
	}

	// Get repository path
	repoPath, err := ps.data.GetProjectRepositoryPath(ctx, run.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("could not get repository path: %w", err)
	}

	// Atomically signal the merge queue and start it if it doesn't exist
	mainBranch := ps.mainBranch()
	mergeQueueID := types.MergeQueueWorkflowID(run.ProjectID)
	item := types.MergeQueueItem{
		RunID:               params.SourceRunID,
		SourceBranchName:    run.BranchName,
		SourceHeadCommitSHA: run.HeadCommitSHA,
		QueuedAt:            time.Now(),
	}
	mqInput := types.MergeQueueWorkflowInput{
		ProjectID:             run.ProjectID,
		RepositoryPath:        repoPath,
		MainBranch:            mainBranch,
		ClaudeConfigPath:      ps.config.Claude.ClaudeJSONHostPath,
		WorkspaceDir:          ps.config.Container.WorkspaceDir,
		OrchestratorTaskQueue: ps.config.Temporal.TaskQueue,
	}
	if _, err := ps.temporal.SignalWithStartWorkflow(ctx, mergeQueueID, workflows.PromoteSignal, item, workflows.MergeQueueWorkflowName, mqInput); err != nil {
		return nil, fmt.Errorf("failed to signal merge queue: %w", err)
	}

	getPipelineLog().Info().
		Str("project_id", run.ProjectID).
		Str("source_run_id", params.SourceRunID).
		Str("merge_queue_id", mergeQueueID).
		Msg("Queued promote signal")

	return &PipelineRunResult{
		RunID:     params.SourceRunID,
		ProjectID: run.ProjectID,
		Name:      fmt.Sprintf("Promote %s", truncateID(params.SourceRunID, 8)),
		Status:    "queued",
	}, nil
}

// GetMergeQueueState queries the merge queue workflow for its current state.
func (ps *PipelineService) GetMergeQueueState(ctx context.Context, projectID string) (*types.MergeQueueState, error) {
	mergeQueueID := types.MergeQueueWorkflowID(projectID)

	var state types.MergeQueueState
	if err := ps.temporal.QueryWorkflow(ctx, mergeQueueID, workflows.MergeQueueStateQuery, &state); err != nil {
		return nil, fmt.Errorf("failed to query merge queue: %w", err)
	}

	return &state, nil
}

// shouldApplyPromptComposition determines if prompt prefix/suffix should be applied.
// Returns true for AI tools (claude, etc.), false for raw command tools (test).
func shouldApplyPromptComposition(steps []models.StepDefinition) bool {
	for _, step := range steps {
		if step.AgentConfig != nil && step.AgentConfig.ToolName == "test" {
			return false
		}
	}
	return true
}
