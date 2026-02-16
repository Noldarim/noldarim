# Noldarim Desktop — Frontend Architecture

**Last Updated**: 2026-02-16
**Architecture Version**: 1.0

## 1. Overview

The desktop client is a **React + TypeScript** single-page application wrapped in **Tauri 2** for cross-platform desktop distribution. It connects to the Noldarim server (`cmd/server`) over REST and WebSocket to orchestrate and monitor AI pipeline runs.

```
┌──────────────────────────────────────────────────────┐
│                  Tauri 2 Shell                        │
│  (Rust native window, CSP enforcement, 1440×920)     │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │           React 19 + Vite 7 + TypeScript 5     │  │
│  │                                                │  │
│  │  ┌──────────┐  ┌──────────┐  ┌─────────────┐  │  │
│  │  │  State    │  │  REST    │  │  WebSocket  │  │  │
│  │  │  (hook)   │  │  Client  │  │  Client     │  │  │
│  │  └────┬─────┘  └────┬─────┘  └──────┬──────┘  │  │
│  │       │              │               │         │  │
│  │  ┌────┴──────────────┴───────────────┴──────┐  │  │
│  │  │              App.tsx (orchestrator)       │  │  │
│  │  └────┬──────────────┬───────────────┬──────┘  │  │
│  │       │              │               │         │  │
│  │  ┌────┴────┐   ┌─────┴─────┐   ┌────┴──────┐  │  │
│  │  │ Sidebar │   │ RunGraph  │   │ Details   │  │  │
│  │  │ Panel   │   │ (xyflow)  │   │ Drawer    │  │  │
│  │  └─────────┘   └───────────┘   └───────────┘  │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
          │                    │
          │  HTTP (REST)       │  WebSocket
          ▼                    ▼
    ┌─────────────────────────────────┐
    │     Noldarim Server             │
    │     (cmd/server, port 8080)     │
    └─────────────────────────────────┘
```

## 2. Technology Stack

| Layer            | Technology                  | Purpose                           |
|------------------|-----------------------------|-----------------------------------|
| Desktop shell    | Tauri 2 (Rust)              | Native window, CSP, packaging     |
| UI framework     | React 19                    | Component rendering               |
| Language         | TypeScript 5 (strict)       | Type safety                       |
| Build            | Vite 7                      | Dev server, HMR, production build |
| Graph rendering  | @xyflow/react               | Pipeline step DAG visualization   |
| State management | Custom React hook            | `useRunStore` (useRef + useState) |
| Testing          | Vitest + @testing-library   | Unit/component tests (jsdom)      |
| Package manager  | Bun                         | Dependency resolution + scripts   |

## 3. Directory Structure

```
desktop/
├── package.json                    # Dependencies, scripts
├── vite.config.ts                  # Vite + Vitest config (port 1420)
├── tsconfig.json                   # TypeScript strict config
├── index.html                      # SPA entry (mounts #root)
│
├── src/
│   ├── main.tsx                    # React DOM mount point
│   ├── App.tsx                     # Root orchestrator component
│   ├── styles.css                  # Design system + all styles
│   ├── vite-env.d.ts               # Vite type declarations
│   │
│   ├── state/
│   │   └── run-store.ts            # useRunStore hook (run lifecycle state)
│   │
│   ├── components/
│   │   ├── ServerSettings.tsx      # Server URL + connect UI
│   │   ├── PipelineForm.tsx        # Pipeline creation form
│   │   ├── RunToolbar.tsx          # Run status bar + cancel
│   │   ├── RunGraph.tsx            # xyflow DAG visualization
│   │   ├── StepDetailsDrawer.tsx   # Right-panel activity inspector
│   │   └── nodes/
│   │       └── StepNode.tsx        # Custom xyflow node for steps
│   │
│   ├── lib/
│   │   ├── types.ts                # Domain models + API types
│   │   ├── api.ts                  # REST client functions
│   │   ├── ws.ts                   # WebSocket connection manager
│   │   ├── obs-mapping.ts          # Activity-to-step mapping
│   │   ├── pipeline-templating.ts  # Template variable substitution
│   │   └── templates.ts            # Built-in pipeline templates
│   │
│   └── test/
│       └── setup.ts                # Vitest env (ResizeObserver polyfill)
│
└── src-tauri/
    ├── Cargo.toml                  # Rust dependencies (tauri 2, serde)
    ├── build.rs                    # Tauri build script
    ├── src/main.rs                 # Default Tauri bootstrap
    └── tauri.conf.json             # Window config, CSP, app identity
```

## 4. Component Architecture

### 4.1 Component Hierarchy

```
App.tsx ─────────────────────────────────────────────────────
│  Owns: server connection, project selection, run lifecycle
│  State: serverUrl, projects, agentDefaults, selectedStepId
│  Refs:  wsRef, pollRef, hydrateTimerRef, connectAbortRef
│
├── <header>
│
├── <aside> (left column — 320px)
│   ├── ServerSettings
│   │     Props: serverUrl, onServerUrlChange, onConnect,
│   │            isConnecting, connectionError
│   │
│   ├── Project selector (<select>)
│   │     Bound to: selectedProjectId
│   │
│   └── PipelineForm
│         Props: templates, disabled, onStart
│         Internal state: selectedTemplate, name, variables, steps
│
├── <section> (center column — 1fr)
│   ├── RunToolbar
│   │     Props: runId, phase, onCancel
│   │
│   └── RunGraph
│         Props: steps, run, activitiesByStep, selectedStepId,
│                onSelectStep
│         Renders: StepNode (custom xyflow node)
│
└── StepDetailsDrawer (fixed right panel — 460px)
      Props: step, events, isOpen, onClose
```

### 4.2 Component Responsibilities

| Component          | Role                                                              |
|--------------------|-------------------------------------------------------------------|
| `App`              | Root orchestrator. Manages connection, hydration, real-time loop. |
| `ServerSettings`   | Server URL input + connect trigger.                               |
| `PipelineForm`     | Template selection, step/variable editing, validation, submit.    |
| `RunToolbar`       | Displays run ID + phase badge. Cancel button.                     |
| `RunGraph`         | Transforms `PipelineRun` + `StepDraft[]` into xyflow DAG.        |
| `StepNode`         | Renders one step: status pill, token stats, diff stats, errors.   |
| `StepDetailsDrawer`| Inspects a selected step's tool activity + event timeline.        |

## 5. State Management

### 5.1 useRunStore Hook

The application uses a custom hook (`useRunStore`) instead of an external state library. It returns `[state, actions]` with stable action references (empty dependency arrays).

```
RunState
├── phase: RunPhase ──────────── "idle" | "starting" | "running" |
│                                "cancelling" | "completed" |
│                                "failed" | "cancelled"
├── runId: string | null
├── projectId: string | null
├── pipelineName: string
├── steps: StepDraft[]
├── run: PipelineRun | null ──── Full server-side run record
├── activities: AIActivityRecord[]
├── activityIds: Set<string> ─── Deduplication guard
└── error: string | null
```

### 5.2 State Transitions

```
                 startRun()
    ┌─────┐    ────────────>    ┌──────────┐
    │ idle │                    │ starting  │
    └─────┘                     └────┬─────┘
       ▲                             │ setRunStarted(runId)
       │ reset()                     ▼
       │                        ┌──────────┐
       ├────────────────────────│ running   │
       │   markCancelling()     └──┬───┬───┘
       │         │                 │   │
       │         ▼                 │   │ setRunData(status=completed)
       │   ┌────────────┐         │   │
       │   │ cancelling  │         │   ▼
       │   └──────┬─────┘         │ ┌───────────┐
       │          │               │ │ completed  │
       │          │ markCancelled()│ └───────────┘
       │          ▼               │
       │   ┌────────────┐        │ markFailed(msg)
       │   │ cancelled   │        │
       │   └────────────┘        ▼
       │                    ┌──────────┐
       └────────────────────│  failed   │
                            └──────────┘
```

### 5.3 Actions Reference

| Action                        | Effect                                              |
|-------------------------------|-----------------------------------------------------|
| `reset()`                     | Returns to idle state, clears all run data           |
| `setError(msg)`               | Sets error message                                   |
| `startRun(projectId, name, steps)` | Transitions to "starting"                       |
| `setRunStarted(runId)`        | Transitions to "running"                             |
| `setRunData(run)`             | Merges PipelineRun, derives phase from run.status    |
| `setActivities(activities[])` | Merges activities by event_id (deduplication)        |
| `appendActivity(activity)`    | Appends single activity (WebSocket streaming)        |
| `markCancelling()`            | Sets phase to "cancelling"                           |
| `markCancelled()`             | Sets phase to "cancelled"                            |
| `markFailed(msg)`             | Sets phase to "failed" with error                    |

### 5.4 State Ownership Map

State that lives **outside** the run store (owned by `App.tsx`):

| State              | Storage       | Purpose                                   |
|--------------------|---------------|-------------------------------------------|
| `serverUrl`        | localStorage  | Persisted server address                  |
| `projects`         | useState      | Project list from GET /projects           |
| `agentDefaults`    | useState      | Default agent config from server          |
| `selectedProjectId`| useState      | Currently selected project                |
| `selectedStepId`   | useState      | Step selected for drawer inspection       |
| `isConnecting`     | useState      | Connection in-progress flag               |
| `connectionError`  | useState      | Connection error message                  |

## 6. Data Flow

### 6.1 Initialization Sequence

```
┌──────────┐     ┌────────┐     ┌──────────────┐
│  App     │     │ api.ts │     │  Server      │
│  mount   │     │        │     │  :8080       │
└────┬─────┘     └────────┘     └──────────────┘
     │                                  │
     │─── GET /api/v1/projects ────────>│
     │<── ProjectsLoadedEvent ──────────│
     │                                  │
     │─── GET /api/v1/agent/defaults ──>│
     │<── AgentDefaultsResponse ────────│
     │                                  │
     │  (user selects project,          │
     │   fills form, clicks Start)      │
     │                                  │
     │─── POST /projects/{id}/pipelines>│
     │<── PipelineRunResult { RunID } ──│
     │                                  │
     │─── GET /pipelines/{runId} ──────>│  ← initial hydration
     │<── PipelineRun ─────────────────│
     │                                  │
     │─── GET /pipelines/{runId}/activity>│
     │<── AIActivityBatchEvent ────────│
     │                                  │
     │═══ WS /ws ═══════════════════════│  ← real-time stream
     │─── subscribe {project_id, run_id}>│
```

### 6.2 Real-Time Update Loop

Once a pipeline is running, two parallel update mechanisms operate:

```
                    ┌─────────────────────────┐
                    │      Server :8080        │
                    └──────┬──────────┬───────┘
                           │          │
              WebSocket    │          │  REST (poll every 10s)
              (streaming)  │          │  + debounced hydration
                           │          │
                    ┌──────┴──────────┴───────┐
                    │        App.tsx           │
                    │                          │
                    │  onEvent(WsEnvelope):    │
                    │   ├─ AIActivityRecord    │
                    │   │  → appendActivity()  │
                    │   └─ any event           │
                    │      → scheduleHydrate() │
                    │         (250ms debounce) │
                    │                          │
                    │  hydrateRun():           │
                    │   ├─ getPipelineRun()    │
                    │   │  → setRunData()      │
                    │   └─ getRunActivity()    │
                    │      → setActivities()   │
                    └─────────────────────────┘
```

**Debounced hydration** (`scheduleHydrate`): prevents flooding the server when many WebSocket events arrive in rapid succession. Collapses multiple triggers into a single REST call after 250ms of quiet.

**Tail hydrations**: after a run completes, two delayed hydrations fire at 2s and 5s to catch any async database writes that may not yet have been committed when the completion event arrived.

**Stale run guard**: `currentRunIdRef` ensures that if the user starts a new run, pending hydrations from a previous run are silently discarded.

### 6.3 Activity-to-Step Mapping

Activities arrive as a flat list scoped to the entire run. The observability layer maps them to individual steps using time windows:

```
mapActivitiesToSteps(run, activities, now)

 Step 1              Step 2              Step 3
 started_at ──────── started_at ──────── started_at ────── now
 │    window 1     │ │    window 2     │ │   window 3    │
 │  events here    │ │  events here    │ │  events here  │
 │  belong to      │ │  belong to      │ │  belong to    │
 │  step 1         │ │  step 2         │ │  step 3       │
 └─────────────────┘ └─────────────────┘ └───────────────┘

Edge cases:
  • Events before first step start  → assigned to first step
  • Events after last window close  → assigned to last started step
  • Skipped steps (status=4)        → excluded from window creation
  • Missing timestamps              → fall back to run start or `now`
```

Returns `StepActivityMap`: `Record<string, AIActivityRecord[]>` keyed by step_id.

### 6.4 Observability Aggregation

Each step's mapped activities are further processed:

```
summarizeStepObservability(events)
  → { eventCount, toolUseCount, toolNames[], inputTokens, outputTokens }

groupToolEvents(events)
  → ToolGroup[] (pairs tool_use with matching tool_result by tool name)
  → Used by StepDetailsDrawer for tool activity display
```

## 7. API Interface Contract

### 7.1 REST Endpoints Used by Desktop

All functions are in `lib/api.ts`. Base URL is configurable (default `http://127.0.0.1:8080`).

| Function               | Method | Path                              | Request Body            | Response Type          |
|-------------------------|--------|-----------------------------------|-------------------------|------------------------|
| `getProjects`           | GET    | `/api/v1/projects`                | —                       | `ProjectsLoadedEvent`  |
| `getAgentDefaults`      | GET    | `/api/v1/agent/defaults`          | —                       | `AgentDefaults`        |
| `startPipeline`         | POST   | `/api/v1/projects/{id}/pipelines` | `StartPipelineRequest`  | `PipelineRunResult`    |
| `getPipelineRun`        | GET    | `/api/v1/pipelines/{runId}`       | —                       | `PipelineRun`          |
| `getPipelineRunActivity`| GET    | `/api/v1/pipelines/{runId}/activity` | —                    | `AIActivityBatchEvent` |
| `cancelPipeline`        | POST   | `/api/v1/pipelines/{runId}/cancel`| `{ reason? }`           | `CancelPipelineResult` |

Error responses follow the shape `{ error: string, context?: string }`.

### 7.2 WebSocket Protocol

**Endpoint**: `ws://<host>/ws` (auto-upgraded from HTTP scheme)

**Client → Server** (subscription management):
```json
{ "type": "subscribe",   "filters": { "project_id": "x", "run_id": "y" } }
{ "type": "unsubscribe", "filters": { "project_id": "x" } }
```

**Server → Client** (event envelope):
```json
{
  "type": "event",
  "event_type": "protocol.PipelineLifecycleEvent",
  "payload": { ... }
}
```

**Connection behavior**:
- Auto-reconnect with exponential backoff: 1s initial, doubling to 30s max
- Close codes 1000 (normal) and 1001 (going away) suppress reconnection
- `disposed` flag prevents zombie reconnects after intentional teardown

## 8. Domain Types

### 8.1 Core Models

```typescript
// Pipeline authoring (client-side)
StepDraft       { id, name, prompt }
PipelineDraft   { name, variables: Record<string,string>, steps: StepDraft[] }

// Pipeline execution (server-side)
PipelineRun     { id, project_id, name, status, step_results[], ... }
StepResult      { step_id, step_index, status, commit_sha, git_diff,
                  files_changed, insertions, deletions,
                  input_tokens, output_tokens, error_message, ... }

// Observability
AIActivityRecord { event_id, run_id, event_type, timestamp,
                   tool_name, tool_input_summary, tool_success,
                   input_tokens, output_tokens, content_preview, ... }
```

### 8.2 Enumerations

```
PipelineRunStatus          StepStatus              RunPhase (client)
  0 = Pending                0 = Pending             "idle"
  1 = Running                1 = Running             "starting"
  2 = Completed              2 = Completed           "running"
  3 = Failed                 3 = Failed              "cancelling"
                             4 = Skipped             "completed"
                                                     "failed"
                                                     "cancelled"

AIEventType
  session_start | session_end | tool_use | tool_result |
  tool_blocked  | thinking    | ai_output | streaming
```

### 8.3 Key Derived Types

```typescript
// View model for xyflow nodes
StepStatusView = "pending" | "running" | "completed" | "failed" | "skipped"

// Activity mapping output
StepActivityMap = Record<string, AIActivityRecord[]>

// Template for pre-built pipelines
PipelineTemplate { id, name, description, draft: PipelineDraft }
```

## 9. Pipeline Templating System

Templates provide pre-built pipeline configurations. Variable substitution uses Go-style `{{ .VarName }}` syntax.

### 9.1 Template Processing Flow

```
PipelineTemplate (templates.ts)
       │
       │  User selects template in PipelineForm
       ▼
PipelineDraft { name, variables, steps }
       │
       │  User edits variables + steps
       │
       │  renderPipelineDraft(draft)
       ▼
RenderedPipeline { name, steps[] }
       │
       │  1. validateTemplateVars() — fail-fast on missing vars
       │  2. renderTemplate() — substitute {{ .VarName }}
       │  3. Runtime vars preserved (RunID, StepIndex, etc.)
       ▼
StartPipelineRequest → POST /projects/{id}/pipelines
```

### 9.2 Runtime Variables (Server-Substituted)

These are **not** validated client-side and pass through to the server:

| Variable           | Resolved at      | Value                          |
|--------------------|------------------|--------------------------------|
| `{{ .RunID }}`     | Pipeline start   | Pipeline run ID                |
| `{{ .StepIndex }}` | Step execution   | 0-based step index             |
| `{{ .StepID }}`    | Step execution   | Step identifier                |
| `{{ .PreviousStepID }}` | Step execution | ID of the preceding step  |

### 9.3 Built-in Templates

| ID                          | Steps | Description                                    |
|-----------------------------|-------|------------------------------------------------|
| `simple-test`               | 2     | File creation and append                       |
| `bug-fix`                   | 3     | Investigate, fix, test                         |
| `feature-implementation`    | 3     | Implement, test, docs (with variables)         |
| `refactor`                  | 3     | Analyze, refactor, cleanup (with variables)    |
| `plan-spec-test-implement`  | 5     | Full cycle with cross-step file refs           |

## 10. Visual Design System

### 10.1 Design Tokens

| Token          | Value                           |
|----------------|---------------------------------|
| Background     | `#f5f2e9` (warm neutral)        |
| Surface        | `#fffdf7` (warm white)          |
| Text primary   | `#1c1917`                       |
| Accent         | `#0f766e` (teal)                |
| Danger         | `#b42318` (red)                 |
| Font (body)    | Space Grotesk (Google Fonts)    |
| Font (mono)    | IBM Plex Mono (Google Fonts)    |
| Border radius  | 10px (panels), 6px (inputs)     |

### 10.2 Layout Grid

```
┌─────────────────────────────────────────────────┐
│  header (app title)                              │
├──────────────┬──────────────────────────────────┤
│              │                                   │
│  aside       │  section                          │
│  320–420px   │  (1fr)                            │
│              │                                   │
│  Server      │  RunToolbar                       │
│  Settings    │  ┌───────────────────────────┐    │
│              │  │                           │    │
│  Project     │  │     RunGraph (460px h)    │    │
│  Selector    │  │     xyflow canvas         │    │
│              │  │                           │    │
│  Pipeline    │  └───────────────────────────┘    │
│  Form        │                                   │
│              │  Error display                    │
├──────────────┴───────────────────┬───────────────┤
│                                  │ StepDetails   │
│                                  │ Drawer        │
│                                  │ (fixed, 460px)│
│                                  │ slides in     │
└──────────────────────────────────┴───────────────┘

Responsive: collapses to single column below 1200px
```

### 10.3 Status Pill Colors

| Status    | Background   | Text      |
|-----------|-------------|-----------|
| pending   | `#e7e5e4`   | `#44403c` |
| running   | `#ccfbf1`   | `#0f766e` |
| completed | `#d1fae5`   | `#065f46` |
| failed    | `#fee2e2`   | `#b42318` |
| skipped   | `#f5f5f4`   | `#78716c` |

## 11. Key Architectural Patterns

### 11.1 Debounced Hydration

WebSocket events trigger `scheduleHydrate()`, which debounces with a 250ms window. This prevents N WebSocket messages from causing N REST roundtrips. A single hydration fetches the full current state.

### 11.2 Activity Deduplication

`activityIds: Set<string>` in the store prevents rendering duplicate events. Both `setActivities` (batch from REST) and `appendActivity` (single from WebSocket) check against this set before inserting.

### 11.3 Tail Hydration

After run completion, two delayed hydrations at +2s and +5s flush any activities that were still being written to the database when the completion event fired. This handles the eventual-consistency window between the orchestrator marking a run complete and all observability records being committed.

### 11.4 Abort Controllers

API requests during connection use an `AbortController` stored in `connectAbortRef`. If the user starts a new connection attempt before the previous one resolves, the old request is aborted to prevent stale state.

### 11.5 Stable Action References

`useRunStore` returns action functions with empty dependency arrays, ensuring they never cause re-renders in consuming components. This is a deliberate alternative to external state libraries.

### 11.6 Form Key Tracking

`PipelineForm` assigns each step an immutable `_key` (via `crypto.randomUUID()`) separate from `step_id`. This ensures correct React reconciliation when steps are reordered or deleted in the form.

## 12. Tauri Integration

The Tauri layer is minimal — it provides:

- **Native window**: 1440×920, resizable, titled "Noldarim Desktop"
- **CSP enforcement**: restricts connections to localhost HTTP/WS + Google Fonts CDN
- **App identity**: `com.noldarim.desktop`, version 0.1.0
- **Build integration**: `npm run build` for production, `npm run dev` for development

No Tauri IPC commands or Rust-side business logic exists currently. The Rust side is a stock bootstrap (`tauri::Builder::default()`). All application logic runs in the webview.

## 13. Testing Strategy

| Scope        | Tool                    | Location                          |
|-------------|-------------------------|-----------------------------------|
| Unit tests  | Vitest                  | `*.test.ts` alongside source      |
| Component   | Vitest + testing-library| `*.test.tsx` alongside source      |
| Environment | jsdom                   | Configured in `vite.config.ts`    |
| Polyfills   | `test/setup.ts`         | ResizeObserver, element dimensions |

Current test coverage targets:
- `obs-mapping.ts` — activity-to-step edge cases (time boundaries, skipped steps)
- `pipeline-templating.ts` — variable substitution and validation
- `RunGraph.tsx` — node rendering and status reflection

## 14. Dependency Graph

```
main.tsx
  └── App.tsx
        ├── state/run-store.ts
        │     └── lib/types.ts
        │
        ├── lib/api.ts
        │     └── lib/types.ts
        │
        ├── lib/ws.ts
        │     └── lib/types.ts
        │
        ├── lib/obs-mapping.ts
        │     └── lib/types.ts
        │
        ├── lib/templates.ts
        │     └── lib/types.ts
        │
        ├── components/ServerSettings.tsx
        │
        ├── components/PipelineForm.tsx
        │     ├── lib/types.ts
        │     └── lib/pipeline-templating.ts
        │           └── lib/types.ts
        │
        ├── components/RunToolbar.tsx
        │     └── lib/types.ts (RunPhase)
        │
        ├── components/RunGraph.tsx
        │     ├── lib/types.ts
        │     ├── lib/obs-mapping.ts (summarizeStepObservability)
        │     ├── components/nodes/StepNode.tsx
        │     └── @xyflow/react
        │
        └── components/StepDetailsDrawer.tsx
              ├── lib/types.ts
              └── lib/obs-mapping.ts (groupToolEvents)
```

All paths lead through `lib/types.ts` as the shared type foundation. No component imports another component except `RunGraph` → `StepNode`. The architecture is deliberately flat — `App.tsx` acts as the single orchestrator, passing data down as props.
