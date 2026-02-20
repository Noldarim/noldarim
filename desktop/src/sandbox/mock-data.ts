// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { PipelineRunStatus, StepStatus } from "../lib/types";
import type { AIActivityRecord, AgentConfigInput, CommitInfo, PipelineRun, RunStepSnapshot, StepResult } from "../lib/types";

// ---------------------------------------------------------------------------
// Random ID helpers
// ---------------------------------------------------------------------------

let counter = 0;

function hex(bytes: number): string {
  const arr = new Uint8Array(bytes);
  crypto.getRandomValues(arr);
  return Array.from(arr, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function mockSha(): string {
  return hex(20); // 40-char SHA
}

export function mockId(): string {
  return `mock-${++counter}-${hex(4)}`;
}

// ---------------------------------------------------------------------------
// Random value helpers
// ---------------------------------------------------------------------------

function randInt(min: number, max: number): number {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

const COMMIT_MESSAGES = [
  "feat: add user authentication flow",
  "fix: resolve null pointer in session handler",
  "refactor: extract validation logic",
  "feat: implement rate limiting middleware",
  "fix: correct timezone handling in scheduler",
  "feat: add WebSocket support for live updates",
  "refactor: simplify database query builder",
  "fix: handle edge case in pagination",
  "feat: add CSV export functionality",
  "feat: implement search with fuzzy matching",
  "fix: prevent race condition in cache invalidation",
  "refactor: migrate to new config format",
  "feat: add dark mode toggle",
  "fix: correct CORS headers for API routes",
  "feat: implement batch processing pipeline",
];

const STEP_NAMES = [
  "Code generation",
  "Code review",
  "Write unit tests",
  "Write integration tests",
  "Refactor pass",
];

const MOCK_PROMPT_TEMPLATES = [
  "You are a senior software engineer. Implement the feature described below.\n\nFeature: {{feature_description}}\nLanguage: {{language}}\nConstraints: {{constraints}}",
  "Review the following code for correctness, performance issues, and security vulnerabilities.\n\nFocus areas: {{focus_areas}}\nStyle guide: {{style_guide}}",
  "Write comprehensive unit tests for the module described below.\n\nModule: {{module_name}}\nFramework: {{test_framework}}\nCoverage target: {{coverage_target}}",
  "Write integration tests that verify the interaction between components.\n\nComponents: {{components}}\nTest scenarios: {{scenarios}}",
  "Refactor the following code to improve readability and maintainability.\n\nGoals: {{refactor_goals}}\nPreserve: {{preserve_behavior}}",
];

const MOCK_TOOL_NAMES = ["claude-code", "aider", "cursor-agent"];
const MOCK_TOOL_VERSIONS = ["1.2.0", "0.9.3", "2.1.0"];

function generateStepSnapshot(runId: string, stepIndex: number): RunStepSnapshot {
  const stepId = `step-${stepIndex}`;
  const stepName = STEP_NAMES[stepIndex % STEP_NAMES.length];
  const toolIdx = stepIndex % MOCK_TOOL_NAMES.length;

  const config: AgentConfigInput = {
    tool_name: MOCK_TOOL_NAMES[toolIdx],
    tool_version: MOCK_TOOL_VERSIONS[toolIdx],
    prompt_template: MOCK_PROMPT_TEMPLATES[stepIndex % MOCK_PROMPT_TEMPLATES.length],
    variables: {
      feature_description: "Add user authentication with JWT tokens",
      language: "TypeScript",
      constraints: "Must be backward compatible with existing auth system",
    },
  };

  return {
    run_id: runId,
    step_id: stepId,
    step_index: stepIndex,
    step_name: stepName,
    agent_config_json: JSON.stringify(config),
    definition_hash: hex(16),
    created_at: new Date().toISOString(),
  };
}

function pickMessage(): string {
  return COMMIT_MESSAGES[randInt(0, COMMIT_MESSAGES.length - 1)];
}

// ---------------------------------------------------------------------------
// Commit chain generator
// ---------------------------------------------------------------------------

export function generateCommitChain(count: number): CommitInfo[] {
  const commits: CommitInfo[] = [];
  for (let i = 0; i < count; i++) {
    const hash = mockSha();
    commits.push({
      Hash: hash,
      Message: pickMessage(),
      Author: i === 0 ? "dev" : "bot",
      Parents: i > 0 ? [commits[i - 1].Hash] : [],
    });
  }
  // Return newest-first (like git log)
  return commits.reverse();
}

// ---------------------------------------------------------------------------
// Step result generator
// ---------------------------------------------------------------------------

function generateStepResult(opts: {
  runId: string;
  stepIndex: number;
  status: StepStatus;
  prevCommitSha: string;
}): StepResult {
  const { runId, stepIndex, status, prevCommitSha } = opts;
  const stepId = `step-${stepIndex}`;
  const isCompleted = status === StepStatus.Completed;
  const commitSha = isCompleted ? mockSha() : "";

  return {
    id: mockId(),
    pipeline_run_id: runId,
    step_id: stepId,
    step_name: STEP_NAMES[stepIndex % STEP_NAMES.length],
    step_index: stepIndex,
    status,
    commit_sha: commitSha || prevCommitSha,
    commit_message: isCompleted ? pickMessage() : "",
    git_diff: "",
    files_changed: isCompleted ? randInt(1, 10) : 0,
    insertions: isCompleted ? randInt(5, 200) : 0,
    deletions: isCompleted ? randInt(0, 50) : 0,
    input_tokens: randInt(500, 5000),
    output_tokens: randInt(200, 3000),
    cache_read_tokens: randInt(0, 1000),
    cache_create_tokens: 0,
    agent_output: isCompleted ? "Step completed successfully" : "",
    duration: isCompleted ? randInt(5, 120) * 1_000_000_000 : 0,
    started_at: "2026-02-18T09:00:00Z",
    completed_at: isCompleted ? "2026-02-18T09:01:00Z" : undefined,
  };
}

// ---------------------------------------------------------------------------
// Run generator
// ---------------------------------------------------------------------------

export type GenerateRunOpts = {
  stepCount: number;
  status: "pending" | "running" | "completed" | "failed";
  baseCommitSha: string;
  parentRunId?: string;
  forkAfterStepId?: string;
};

export type GenerateRunResult = {
  run: PipelineRun;
  commits: CommitInfo[];
  activities: AIActivityRecord[];
};

const STATUS_MAP: Record<string, PipelineRunStatus> = {
  pending: PipelineRunStatus.Pending,
  running: PipelineRunStatus.Running,
  completed: PipelineRunStatus.Completed,
  failed: PipelineRunStatus.Failed,
};

export function generateRun(opts: GenerateRunOpts): GenerateRunResult {
  const runId = mockId();
  const runStatus = STATUS_MAP[opts.status];
  const stepResults: StepResult[] = [];
  const commits: CommitInfo[] = [];
  let prevSha = opts.baseCommitSha;

  for (let i = 0; i < opts.stepCount; i++) {
    let stepStatus: StepStatus;
    if (opts.status === "completed") {
      stepStatus = StepStatus.Completed;
    } else if (opts.status === "pending") {
      stepStatus = StepStatus.Pending;
    } else if (opts.status === "failed") {
      stepStatus = i < opts.stepCount - 1 ? StepStatus.Completed : StepStatus.Failed;
    } else {
      // running — first N-1 completed, last one running
      if (i < opts.stepCount - 1) {
        stepStatus = StepStatus.Completed;
      } else {
        stepStatus = StepStatus.Running;
      }
    }

    // If this step is in a fork and before the fork point, mark as skipped
    if (opts.forkAfterStepId && `step-${i}` <= opts.forkAfterStepId) {
      stepStatus = StepStatus.Skipped;
    }

    const sr = generateStepResult({
      runId,
      stepIndex: i,
      status: stepStatus,
      prevCommitSha: prevSha,
    });
    stepResults.push(sr);

    if (sr.commit_sha && sr.commit_sha !== prevSha) {
      commits.push({
        Hash: sr.commit_sha,
        Message: sr.commit_message,
        Author: "bot",
        Parents: [prevSha],
      });
      prevSha = sr.commit_sha;
    }
  }

  const headSha = runStatus === PipelineRunStatus.Completed ? prevSha : "";

  const stepSnapshots: RunStepSnapshot[] = [];
  for (let i = 0; i < opts.stepCount; i++) {
    stepSnapshots.push(generateStepSnapshot(runId, i));
  }

  const run: PipelineRun = {
    id: runId,
    project_id: "sandbox-project",
    name: `Pipeline ${runId.slice(0, 8)}`,
    status: runStatus,
    base_commit_sha: opts.baseCommitSha,
    start_commit_sha: opts.baseCommitSha,
    head_commit_sha: headSha,
    parent_run_id: opts.parentRunId,
    fork_after_step_id: opts.forkAfterStepId,
    created_at: new Date(Date.now() - randInt(0, 3600_000)).toISOString(),
    started_at: new Date(Date.now() - randInt(0, 3600_000)).toISOString(),
    completed_at: runStatus === PipelineRunStatus.Completed
      ? new Date().toISOString()
      : undefined,
    step_results: stepResults,
    step_snapshots: stepSnapshots,
  };

  return { run, commits, activities: [] };
}

// ---------------------------------------------------------------------------
// Scenario presets
// ---------------------------------------------------------------------------

export type ScenarioData = {
  runs: PipelineRun[];
  commits: CommitInfo[];
  runDetails: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }>;
};

export function generateScenario(name: string): ScenarioData {
  // Reset counter for reproducibility within a scenario
  counter = 0;

  switch (name) {
    case "single-completed":
      return singleCompleted();
    case "running-3-steps":
      return running3Steps();
    case "3-runs-1-fork":
      return threeRunsOneFork();
    case "complex-5-runs":
      return complex5Runs();
    case "stress-10-runs":
      return stress10Runs();
    default:
      return singleCompleted();
  }
}

export const SCENARIO_NAMES = [
  "single-completed",
  "running-3-steps",
  "3-runs-1-fork",
  "complex-5-runs",
  "stress-10-runs",
] as const;

function singleCompleted(): ScenarioData {
  const baseChain = generateCommitChain(2);
  const baseSha = baseChain[baseChain.length - 1].Hash;
  const result = generateRun({ stepCount: 2, status: "completed", baseCommitSha: baseSha });
  return buildScenario([result], baseChain);
}

function running3Steps(): ScenarioData {
  const baseChain = generateCommitChain(2);
  const baseSha = baseChain[baseChain.length - 1].Hash;
  const result = generateRun({ stepCount: 3, status: "running", baseCommitSha: baseSha });
  return buildScenario([result], baseChain);
}

function threeRunsOneFork(): ScenarioData {
  const baseChain = generateCommitChain(2);
  const baseSha = baseChain[baseChain.length - 1].Hash;

  const r1 = generateRun({ stepCount: 2, status: "completed", baseCommitSha: baseSha });
  const r1HeadSha = r1.run.head_commit_sha || baseSha;

  const r2 = generateRun({ stepCount: 3, status: "running", baseCommitSha: r1HeadSha });

  // Fork from r1 after step-0
  const r3 = generateRun({
    stepCount: 2,
    status: "completed",
    baseCommitSha: baseSha,
    parentRunId: r1.run.id,
    forkAfterStepId: "step-0",
  });

  return buildScenario([r1, r2, r3], baseChain);
}

function complex5Runs(): ScenarioData {
  const baseChain = generateCommitChain(3);
  const baseSha = baseChain[baseChain.length - 1].Hash;

  const r1 = generateRun({ stepCount: 2, status: "completed", baseCommitSha: baseSha });
  const r1Head = r1.run.head_commit_sha || baseSha;

  const r2 = generateRun({ stepCount: 3, status: "completed", baseCommitSha: r1Head });
  const r2Head = r2.run.head_commit_sha || r1Head;

  const r3 = generateRun({ stepCount: 2, status: "failed", baseCommitSha: r2Head });

  const r4 = generateRun({ stepCount: 4, status: "running", baseCommitSha: r2Head });

  const r5 = generateRun({
    stepCount: 3,
    status: "completed",
    baseCommitSha: baseSha,
    parentRunId: r1.run.id,
    forkAfterStepId: "step-0",
  });

  return buildScenario([r1, r2, r3, r4, r5], baseChain);
}

function stress10Runs(): ScenarioData {
  const baseChain = generateCommitChain(3);
  const baseSha = baseChain[baseChain.length - 1].Hash;
  const results: GenerateRunResult[] = [];

  let prevHead = baseSha;
  for (let i = 0; i < 10; i++) {
    const statuses = ["completed", "running", "failed", "completed"] as const;
    const status = statuses[i % statuses.length];
    const stepCount = randInt(1, 5);
    const r = generateRun({ stepCount, status, baseCommitSha: prevHead });
    results.push(r);
    if (r.run.head_commit_sha) {
      prevHead = r.run.head_commit_sha;
    }
  }

  return buildScenario(results, baseChain);
}

function buildScenario(results: GenerateRunResult[], baseChain: CommitInfo[]): ScenarioData {
  const allCommits = [...baseChain];
  const runs: PipelineRun[] = [];
  const runDetails: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }> = {};

  for (const r of results) {
    runs.push(r.run);
    allCommits.push(...r.commits);
    runDetails[r.run.id] = { run: r.run, activities: r.activities };
  }

  return { runs, commits: allCommits, runDetails };
}
