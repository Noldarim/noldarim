export type Project = {
  id: string;
  name: string;
  description: string;
  repository_path: string;
};

export type ProjectsLoadedEvent = {
  Projects: Record<string, Project>;
};

export type AgentDefaults = {
  tool_name: string;
  tool_version: string;
  flag_format: "space" | "equals" | string;
  tool_options: Record<string, unknown>;
};

export type AgentConfigInput = {
  tool_name: string;
  tool_version?: string;
  prompt_template: string;
  variables?: Record<string, string>;
  tool_options?: Record<string, unknown>;
  flag_format?: string;
};

export type StepInputRequest = {
  step_id: string;
  name: string;
  agent_config: AgentConfigInput;
};

export type StartPipelineRequest = {
  name: string;
  steps: StepInputRequest[];
  base_commit_sha?: string;
  fork_from_run_id?: string;
  fork_after_step_id?: string;
  no_auto_fork?: boolean;
};

export type PipelineRunResult = {
  RunID: string;
  ProjectID: string;
  Name: string;
  WorkflowID: string;
  AlreadyExists?: boolean;
  Status?: string;
  ForkFromRunID?: string;
  ForkAfterStepID?: string;
  SkippedSteps?: number;
};

export const PipelineRunStatus = {
  Pending: 0,
  Running: 1,
  Completed: 2,
  Failed: 3
} as const;
export type PipelineRunStatus = (typeof PipelineRunStatus)[keyof typeof PipelineRunStatus];

export const StepStatus = {
  Pending: 0,
  Running: 1,
  Completed: 2,
  Failed: 3,
  Skipped: 4
} as const;
export type StepStatus = (typeof StepStatus)[keyof typeof StepStatus];

export type StepResult = {
  id: string;
  pipeline_run_id: string;
  step_id: string;
  step_index: number;
  status: StepStatus;
  commit_sha: string;
  commit_message: string;
  git_diff: string;
  files_changed: number;
  insertions: number;
  deletions: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_create_tokens: number;
  agent_output: string;
  duration: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
};

export type PipelineRun = {
  id: string;
  project_id: string;
  name: string;
  status: PipelineRunStatus;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  completed_at?: string;
  error_message?: string;
  step_results?: StepResult[];
};

export type AIEventType =
  | "session_start"
  | "session_end"
  | "tool_use"
  | "tool_result"
  | "tool_blocked"
  | "thinking"
  | "ai_output"
  | "streaming"
  | "error"
  | "stop"
  | "subagent_start"
  | "subagent_stop"
  | "user_prompt"
  | string;

export type AIActivityRecord = {
  event_id: string;
  task_id: string;
  run_id: string;
  step_id?: string;
  event_type: AIEventType;
  timestamp: string;
  tool_name?: string;
  tool_input_summary?: string;
  tool_success?: boolean | null;
  tool_error?: string;
  content_preview?: string;
  input_tokens?: number;
  output_tokens?: number;
  cache_read_tokens?: number;
  cache_create_tokens?: number;
  raw_payload?: string;
};

export type AIActivityBatchEvent = {
  TaskID: string;
  ProjectID: string;
  Activities: AIActivityRecord[];
};

export type CancelPipelineResult = {
  RunID: string;
  Reason: string;
  WorkflowStatus: string;
};

export type WsEnvelope = {
  type: "event" | "error";
  event_type?: string;
  payload?: unknown;
  message?: string;
};

export type PipelineLifecycleEvent = {
  Type: "created" | "step_started" | "step_completed" | "step_failed" | "finished" | "failed";
  ProjectID: string;
  RunID: string;
  StepID?: string;
  StepIndex?: number;
  StepName?: string;
};

export type StepDraft = {
  id: string;
  name: string;
  prompt: string;
};

export type PipelineDraft = {
  name: string;
  variables: Record<string, string>;
  steps: StepDraft[];
};

export type StepStatusView = "pending" | "running" | "completed" | "failed" | "skipped";

export function pipelineStatusToPhase(status: PipelineRunStatus): "running" | "completed" | "failed" {
  if (status === PipelineRunStatus.Completed) {
    return "completed";
  }
  if (status === PipelineRunStatus.Failed) {
    return "failed";
  }
  return "running";
}

export function stepStatusToView(status: StepStatus): StepStatusView {
  switch (status) {
    case StepStatus.Running:
      return "running";
    case StepStatus.Completed:
      return "completed";
    case StepStatus.Failed:
      return "failed";
    case StepStatus.Skipped:
      return "skipped";
    default:
      return "pending";
  }
}
