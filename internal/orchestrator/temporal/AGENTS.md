# Temporal Workflows & Activities

## Overview

Temporal SDK orchestration layer. Defines all workflows (pipeline, setup, step processing, observability) and activities (git, container, data, AI events). Two queue boundaries: orchestrator queue and per-run agent queue.

## Structure

```
temporal/
├── client.go              # Temporal client wrapper (connect, start, signal, query)
├── workflows/
│   ├── pipeline.go        # PipelineWorkflow — top-level orchestrator (647 lines)
│   ├── setup.go           # SetupWorkflow — worktree + container + credentials
│   ├── processing_step.go # ProcessingStepWorkflow — single step execution
│   ├── ai_observability.go # AIObservabilityWorkflow — transcript watch + event stream
│   ├── promote.go         # PromoteWorkflow — merge step results (513 lines)
│   ├── merge_queue.go     # MergeQueueWorkflow
│   ├── compensation.go    # Cleanup/compensation for failed workflows
│   ├── process_task.go    # LEGACY — single-task execution (use PipelineWorkflow)
│   └── create_task.go     # Task creation flow
├── activities/
│   ├── git.go             # Worktree, commit, diff, push (575 lines)
│   ├── local_exec.go      # Execute commands inside containers (511 lines)
│   ├── container.go       # Create/start/stop Docker containers
│   ├── ai_events.go       # Save/parse/update AI activity events
│   ├── transcript_watcher.go # Watch transcript files in containers
│   ├── data.go            # DB CRUD (save tasks, runs, step results)
│   ├── events.go          # Emit protocol lifecycle events
│   ├── agent_helpers.go   # Build agent CLI commands
│   ├── agent_setup.go     # Copy Claude config/credentials
│   ├── pipeline_data.go   # Pipeline-specific data activities
│   ├── step_documentation.go # Generate step docs/history
│   ├── task_file.go       # Write task description files
│   └── merge_queue.go     # Merge queue operations
├── types/                 # All workflow/activity I/O structs
├── utils/
│   ├── config.go          # Activity options factory
│   └── taskqueue.go       # Task queue name generation
└── workers/
    └── worker.go          # ALL workflow/activity registration
```

## Workflow Graph

```
StartPipeline/CreateTask → PipelineWorkflow
  ├── SetupWorkflow (worktree + container + credentials + compensation)
  ├── AIObservabilityWorkflow (child, pipeline-scoped, on agent queue)
  │   └── WatchTranscript → SaveRaw → Parse → Update → Publish
  └── ProcessingStepWorkflow (per step, sequential)
      ├── PrepareAgentCommand + LocalExecuteActivity
      ├── CaptureGitDiff + GitCommit + StepDocumentation
      └── GetTokenTotals + EmitStepComplete
```

## Queue Boundaries

| Queue | Runs On | Activities |
|-------|---------|------------|
| `noldarim-task-queue` | Host (orchestrator worker) | All workflows + DB/git/container activities |
| `task-queue-<runID>` | Container (`cmd/agent`) | LocalExecuteActivity, WatchTranscriptActivity |

## Where to Look

| Task | Location |
|------|----------|
| Add new activity | `activities/` + register in `workers/worker.go` |
| Add agent-queue activity | `activities/` + register in BOTH `workers/worker.go` AND `cmd/agent/main.go` |
| Modify pipeline flow | `workflows/pipeline.go` |
| Change setup sequence | `workflows/setup.go` (includes compensation) |
| Add workflow signal | `types/ai_events.go` or `types/event_inputs.go` |
| Configure activity options | `utils/config.go` (factory pattern) |

## Conventions

- Activity options: use `utils/config.go` factory — never inline `workflow.ActivityOptions{}`
- Workflow I/O: all structs in `types/` — never pass raw primitives across workflow boundaries
- SetupWorkflow: compensation pattern — register cleanup for each created resource
- Pipeline fork: skips completed steps, reuses parent worktree state via RunStepSnapshot
- Signals: `AIEventSignal` for streaming AI events, cancellation signals for abort
- ProcessTaskWorkflow: LEGACY — new code uses PipelineWorkflow with single step

## Anti-Patterns

- **Direct activity calls from PipelineWorkflow**: Delegate to child workflows (Setup, ProcessingStep)
- **Missing compensation**: Never create containers/worktrees without cleanup registration
- **Dual registration**: Agent-queue activities must be in `workers/worker.go` AND `cmd/agent/main.go`
- **Stale agent image**: Rebuild Docker image after ANY change to agent-queue code
- **Inline activity options**: Use `utils/config.go` factory, not ad-hoc `workflow.ActivityOptions{}`
