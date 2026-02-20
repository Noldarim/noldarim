import { z } from "zod/v4";

import type { PipelineRunStatus, StepStatus } from "./types";

// --- Status schemas (narrow numeric unions matching types.ts) ---

const PipelineRunStatusSchema = z.union([z.literal(0), z.literal(1), z.literal(2), z.literal(3)]) as z.ZodType<PipelineRunStatus>;

const StepStatusSchema = z.union([z.literal(0), z.literal(1), z.literal(2), z.literal(3), z.literal(4)]) as z.ZodType<StepStatus>;

// --- Atomic schemas ---

export const ProjectSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  repository_path: z.string()
});

export const ProjectsLoadedEventSchema = z.object({
  Projects: z.record(z.string(), ProjectSchema).nullable().transform((v) => v ?? {})
});

export const AgentDefaultsSchema = z.object({
  tool_name: z.string(),
  tool_version: z.string(),
  flag_format: z.string(),
  tool_options: z.record(z.string(), z.unknown())
});

export const StepResultSchema = z.object({
  id: z.string(),
  pipeline_run_id: z.string(),
  step_id: z.string(),
  step_name: z.string().optional(),
  step_index: z.number(),
  status: StepStatusSchema,
  commit_sha: z.string(),
  commit_message: z.string(),
  git_diff: z.string(),
  files_changed: z.number(),
  insertions: z.number(),
  deletions: z.number(),
  input_tokens: z.number(),
  output_tokens: z.number(),
  cache_read_tokens: z.number(),
  cache_create_tokens: z.number(),
  agent_output: z.string(),
  duration: z.number(),
  error_message: z.string().optional(),
  started_at: z.string().optional(),
  completed_at: z.string().optional()
});

export const RunStepSnapshotSchema = z.object({
  run_id: z.string(),
  step_id: z.string(),
  step_index: z.number(),
  step_name: z.string(),
  agent_config_json: z.string(),
  definition_hash: z.string(),
  created_at: z.string().optional()
});

export const PipelineRunSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  status: PipelineRunStatusSchema,
  base_commit_sha: z.string().optional(),
  start_commit_sha: z.string().optional(),
  head_commit_sha: z.string().optional(),
  parent_run_id: z.string().optional(),
  fork_after_step_id: z.string().optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
  started_at: z.string().optional(),
  completed_at: z.string().optional(),
  error_message: z.string().optional(),
  step_results: z.array(StepResultSchema).optional(),
  step_snapshots: z.array(RunStepSnapshotSchema).optional()
});

export const PipelineRunsLoadedEventSchema = z.object({
  ProjectID: z.string(),
  ProjectName: z.string(),
  RepositoryPath: z.string(),
  Runs: z.record(z.string(), PipelineRunSchema).nullable().transform((v) => v ?? {})
});

export const CommitInfoSchema = z.object({
  Hash: z.string(),
  Message: z.string(),
  Author: z.string(),
  Parents: z.array(z.string())
});

export const CommitsLoadedEventSchema = z.object({
  ProjectID: z.string(),
  RepositoryPath: z.string(),
  Commits: z.array(CommitInfoSchema).nullable().transform((v) => v ?? [])
});

export const PipelineRunResultSchema = z.object({
  RunID: z.string(),
  ProjectID: z.string(),
  Name: z.string(),
  WorkflowID: z.string(),
  AlreadyExists: z.boolean().optional(),
  Status: z.string().optional(),
  ForkFromRunID: z.string().optional(),
  ForkAfterStepID: z.string().optional(),
  SkippedSteps: z.number().optional()
});

export const AIActivityRecordSchema = z.object({
  event_id: z.string(),
  task_id: z.string(),
  run_id: z.string(),
  step_id: z.string().optional(),
  event_type: z.string(),
  timestamp: z.string(),
  tool_name: z.string().optional(),
  tool_input_summary: z.string().optional(),
  tool_success: z.union([z.boolean(), z.null()]).optional(),
  tool_error: z.string().optional(),
  content_preview: z.string().optional(),
  input_tokens: z.number().optional(),
  output_tokens: z.number().optional(),
  cache_read_tokens: z.number().optional(),
  cache_create_tokens: z.number().optional(),
  raw_payload: z.string().optional()
});

export const AIActivityBatchEventSchema = z.object({
  TaskID: z.string(),
  ProjectID: z.string(),
  Activities: z.array(AIActivityRecordSchema).nullable().transform((v) => v ?? [])
});

export const WsEnvelopeSchema = z.object({
  type: z.enum(["event", "error"]),
  event_type: z.string().optional(),
  payload: z.unknown().optional(),
  message: z.string().optional()
});

export const CancelPipelineResultSchema = z.object({
  RunID: z.string(),
  Reason: z.string(),
  WorkflowStatus: z.string()
});
