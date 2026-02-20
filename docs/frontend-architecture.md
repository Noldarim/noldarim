# Noldarim Desktop — Frontend Architecture

**Last Updated**: 2026-02-20
**Architecture Version**: 3.0 (DAG-based graph with step expansion)

## 1. Overview

The desktop app is a React + TypeScript SPA inside Tauri 2.  
It is server-first: the UI uses REST + WebSocket from `cmd/server`.

The graph UX uses a **DAG-based layout with always-expanded step chains**:

- Commits are positioned via Kahn's topological sort (depth = column, pipeline = row).
- All runs are always expanded into step chains: patch nodes (step config) and outcome nodes (step result).
- Pipeline background nodes visually group each run's step chain.
- Pipeline/step execution state lives on edges between patch and outcome nodes.
- A right-side `EdgeDetailsDrawer` is the primary inspection/edit surface.

## 2. Technology

- React 19, TypeScript 5, Vite 7
- Zustand state stores
- `@xyflow/react` graph rendering
- Vitest + Testing Library
- Tauri 2 shell

## 3. Key Frontend Modules

### 3.1 Root Composition

- `desktop/src/App.tsx`
  - Owns server connection, project selection, pipeline start/cancel, base commit selection.
  - Renders:
    - `NoldarimGraphView` (main graph canvas)
    - `FloatingProjectSelector` (project picker overlay)
    - `FloatingRunStatus` (live run status overlay, conditional)
    - `PipelineFormDialog` (pipeline configuration dialog)
    - `DevNavToggle` (sandbox navigation, DEV only)

### 3.2 Graph + Drawer

- `desktop/src/components/NoldarimGraphView.tsx`
  - Fetches runs and commit history.
  - Merges live run overlay from run store.
  - Maintains graph selection:
    - `run-edge`
    - `step-edge`
  - Renders custom ReactFlow nodes/edges.
  - Opens `EdgeDetailsDrawer`.

- `desktop/src/components/EdgeDetailsDrawer.tsx`
  - Tabs:
    - Overview
    - Metrics
    - Diff
    - Logs
    - Step Config
  - Actions:
    - Cancel run
    - Rerun from source commit
    - Fork from selected step (editable config)

### 3.3 Graph Layout Engine

- `desktop/src/lib/graph-layout.ts`
  - Builds graph from commits + runs + run details via `buildProjectGraph()`.
  - Pipeline: `resolveRuns` → `buildDAG` → `collectCommitShas` → `computeDepths` → `assignPipelineRows` → `spreadBlockedHeads` → `computeSourceNudges` → `computeNodePositions` → `createEdges`.
  - Depth (x-axis) via Kahn's topological sort; row (y-axis) per pipeline.
  - All runs always expanded into step chains: patch nodes (config) + outcome nodes (result).
  - Shared helper `stepOutcomeKey()` for consistent synthetic/real SHA resolution.
  - `computeNodePositions()` delegates to: `createCommitNodes`, `createGhostNodes`, `createStepChainNodes`, `createPipelineBgNodes`, `markForkPoints`.
  - `createEdges()` delegates step chain building to `buildStepChainEdges()`.
  - Ghost endpoint nodes for legacy runs without step data.

### 3.4 Graph Node/Edge Components

- `desktop/src/components/nodes/CommitNode.tsx` — commit and step outcome nodes
- `desktop/src/components/nodes/PatchNode.tsx` — step config/input nodes
- `desktop/src/components/nodes/PipelineBgNode.tsx` — pipeline background containers
- `desktop/src/components/edges/PipelineEdge.tsx` — all edge types (run, step, connector)

## 4. State Model

### 4.1 Run State (live execution)

- `desktop/src/state/run-store.ts`
- Holds:
  - `phase`, `connectionStatus`, `runId`, `projectId`
  - `runDefinition` (steps + pipeline name)
  - live `run` snapshot
  - live `stepExecutionById`
  - `activityByEventId` and `activityByStepId`
  - `error`
- Updated by WebSocket + hydration via `desktop/src/hooks/useRunConnection.ts`.

### 4.2 Project Graph State (historical + merged graph data)

- `desktop/src/state/project-graph-store.ts`
- Holds:
  - project runs list
  - per-run detail cache (`expandedRunData`: run + activities)
  - loading/error + refresh token

## 5. API Surface Used by Desktop

From `desktop/src/lib/api.ts`:

- `GET /api/v1/projects`
- `GET /api/v1/agent/defaults`
- `GET /api/v1/projects/{id}/commits?limit=...`
- `GET /api/v1/projects/{id}/pipelines`
- `GET /api/v1/pipelines/{runId}`
- `GET /api/v1/pipelines/{runId}/activity`
- `POST /api/v1/projects/{id}/pipelines`
- `POST /api/v1/pipelines/{runId}/cancel`

## 6. Data Flow (Current)

### 6.1 Baseline graph load

1. Load runs for selected project.
2. Load commit history:
   - no runs: limit 4
   - with runs: limit 200
3. Build graph from commit spine + run edges.

### 6.2 Edge selection

All runs are always expanded into step chains — there is no collapse toggle.

1. Step edge click:
   - selection becomes `step-edge`
   - run details fetched if not cached (`getPipelineRun` + `getPipelineRunActivity`)
   - drawer opens on config/logs/diff for that step.
2. Run tail edge click:
   - selection becomes `run-edge`
   - drawer shows run-level summary and metrics.

### 6.3 Live run overlay

When `run-store` is in live phase for the selected project:

- live run is merged into the project run set
- live activities are merged into run details
- graph always shows current progress without waiting for list refresh

### 6.4 Fork from step

From drawer:

1. Parse step snapshots from `run.step_snapshots`.
2. Edit selected step config.
3. Start pipeline with deterministic fork payload:
   - `fork_from_run_id`
   - `fork_after_step_id` (previous step)
   - `no_auto_fork = true`
   - `base_commit_sha` from source run

## 7. Frontend Types (Important Changes)

In `desktop/src/lib/types.ts`:

- Added:
  - `CommitInfo`
  - `CommitsLoadedEvent`
  - `RunStepSnapshot`
- Extended:
  - `PipelineRun.step_snapshots?: RunStepSnapshot[]`

In `desktop/src/lib/schemas.ts`:

- Added `CommitInfoSchema`, `CommitsLoadedEventSchema`, `RunStepSnapshotSchema`
- Extended `PipelineRunSchema` to include `step_snapshots`

## 8. Styling

Modular token-based CSS architecture (split from monolithic `styles.css`):

- `_tokens.css` — design tokens (colors, spacing, typography)
- `_reset.css` — normalize/reset
- `_base.css` — app shell layout
- `_nodes.css` — commit, patch, pipeline-bg node styles; ghost/step/fork-point states
- `_drawer.css` — edge details drawer, tabs, config form, log cards
- `_components.css` — form/dialog styles
- `_layout.css` — graph canvas layout
- `_animations.css` — transitions and keyframes
- `_vendor.css` — reactflow overrides, edge label cards, status visual states
- `_sandbox.css` — development sandbox styles (DEV only)
- `index.css` — import aggregator

## 9. Testing

- Graph layout unit tests (95 tests):
  - `desktop/src/lib/graph-layout.test.ts`
- Graph component integration tests:
  - `desktop/src/components/NoldarimGraphView.test.tsx`
  - `desktop/src/components/EdgeDetailsDrawer.test.tsx`
- Existing run store, schema, mapping, and duration tests remain active.

## 10. Legacy Components

Legacy graph components were removed from the active frontend surface to avoid drift:

- `StepDetailsDrawer`
- `StepNode`
- `RunNode`

Primary graph surface is `NoldarimGraphView` + `EdgeDetailsDrawer`.
