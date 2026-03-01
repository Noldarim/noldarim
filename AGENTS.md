# Project Memory

**Generated:** 2026-03-01 | **Commit:** 7c591bc | **Branch:** main

## Overview

Noldarim: workflow orchestrator for AI-assisted software tasks. Go backend (Temporal + Docker + SQLite) with Tauri+React desktop UI. Server-first architecture — desktop client calls REST/WebSocket APIs exposed by `cmd/server`.

## Structure

```
.
├── cmd/
│   ├── server/         # PRIMARY entry point — REST API + WebSocket (port 8080)
│   ├── agent/          # Temporal worker — runs inside task containers
│   ├── noldarim/       # CLI tool (operator workflows)
│   ├── migrate/        # DB migration utility
│   ├── app/            # LEGACY TUI — deprecated as primary entry
│   └── dev/            # 20+ dev tools (TUI demos, adapters, DB explorer, obs harness)
├── internal/
│   ├── orchestrator/   # Core business logic — see internal/orchestrator/AGENTS.md
│   │   ├── temporal/   # Temporal workflows/activities — see temporal/AGENTS.md
│   │   ├── services/   # Business services (pipeline, data, git)
│   │   ├── agents/     # AI agent adapters (Claude, test)
│   │   ├── database/   # GORM persistence layer
│   │   └── models/     # Domain models
│   ├── server/         # HTTP server (chi), WebSocket broadcaster, middleware
│   ├── protocol/       # Command/Event contracts (shared between all runtimes)
│   ├── aiobs/          # AI observability — transcript parsing adapters
│   ├── logger/         # Zerolog factory with per-package levels
│   ├── config/         # Viper-based config (config.yaml + NOLDARIM_* env vars)
│   ├── cli/            # CLI command implementations
│   ├── tui/            # DEPRECATED terminal UI (Charmbracelet)
│   └── common/         # Shared metadata
├── desktop/            # Tauri + React desktop app — see desktop/AGENTS.md
├── pkg/containers/     # Docker container abstraction (client, service, validation)
├── api/                # OpenAPI spec (openapi.yaml)
├── docker/             # Dockerfiles (agent image, firewall container)
├── docs/               # Architecture docs
├── scripts/            # Build/test shell scripts
├── tests/integration/  # Go integration tests (Docker + Temporal)
└── frontend/           # UNUSED — delete candidate (only node_modules)
```

## Where to Look

| Task | Location | Notes |
|------|----------|-------|
| Add API endpoint | `internal/server/handlers.go` + `server.go` (routes) | Chi router, follow existing handler pattern |
| Add workflow step | `internal/orchestrator/temporal/workflows/` | See temporal/AGENTS.md |
| Add Temporal activity | `internal/orchestrator/temporal/activities/` | Register in `workers/worker.go` |
| Modify pipeline logic | `internal/orchestrator/services/pipeline_service.go` | Thin orchestrator delegates here |
| Desktop UI component | `desktop/src/components/` | React + ReactFlow |
| Event/command contract | `internal/protocol/` | Commands = UI->orchestrator, Events = orchestrator->UI |
| Database schema | `internal/orchestrator/models/gorm_models.go` | GORM auto-migration |
| Container management | `pkg/containers/` | Docker SDK wrapper |
| AI transcript parsing | `internal/aiobs/adapters/claude/` | Claude-specific adapter |
| Config defaults | `config.yaml` | Override with `NOLDARIM_*` env vars |

## Data Flow

```
Desktop → HTTP/WS → cmd/server → internal/server → PipelineService → Temporal
                                                                        ↓
                                                              Workflow → Activities → Docker Container
                                                                        ↓
                                                              AI Agent → Transcript → AIObservability
```

In server-first mode, REST handlers call services directly. `cmdChan` idles (exists for legacy TUI compat).

## Protocol Contracts

`internal/protocol/` defines all Command/Event types shared across runtimes:

- **Commands** (UI → orchestrator): `CreateTaskCommand`, `StartPipelineCommand`, `LoadProjectsCommand`, etc.
- **Events** (orchestrator → UI): `PipelineRunStartedEvent`, `TaskLifecycleEvent`, `AIActivityBatchEvent`, etc.
- **Lifecycle families**: Task, Pipeline, AIActivity, Error
- Server-mode: REST does direct service calls; lifecycle events still flow via `eventChan` → WebSocket

## Conventions

- **No enforced linting**: No ESLint, Prettier, golangci-lint configs — rely on Go defaults and IDE
- **Copyright header**: All Go files: `// Copyright (C) 2025-2026 Noldarim` + `// SPDX-License-Identifier: AGPL-3.0-or-later`
- **Logging**: Per-package loggers via `logger.Get*Logger()` — never raw `fmt.Print` or `log.*`
- **Error handling**: Return `fmt.Errorf("context: %w", err)` — no bare error returns
- **Config override**: `config.yaml` values → overridden by `NOLDARIM_*` env vars (underscore-separated)
- **Test assertions**: Go uses `testify` (require/assert). Desktop uses Vitest + Testing Library
- **Test naming**: Go `*_test.go` colocated. TS `*.test.ts(x)` colocated or in `__tests__/`
- **Package manager**: Desktop supports both `bun` (preferred) and `npm` (fallback)

## Anti-Patterns (This Project)

- **Stale agent runtime**: Rebuild and restart AI agent/worker whenever workflow logic, observability/event parsing, or step-context propagation code changes. Runtime behavior stays stale otherwise.
- **Adapter registration**: Call `RegisterAll()` before `Get()` on AI observability adapters — nil return otherwise
- **TUI height**: Use `MaxHeight()` not `Height()` alone for layout constraints (Height is minimum, not ceiling)
- **Deprecated test helper**: Use `testutil.WithGitService()` not `createTestGitService()`
- **Docker exit code**: Currently hardcoded to 0 — cannot distinguish success/failure container exits

## Commands

```bash
# Prerequisites
temporal server start-dev     # Required: Temporal server on 127.0.0.1:7233
make build-agent              # Required: Build Docker agent image

# Primary runtime
make run                      # Start API server (port 8080)
make build-server             # Compile server binary

# Desktop
make desktop-dev              # Tauri dev mode (requires backend running)
make desktop-web              # Frontend-only Vite dev (port 1420)
make desktop-build            # Production build
make desktop-test             # Vitest

# Testing
make test                     # Go tests + desktop tests
make test-tui                 # Legacy TUI tests only

# Utilities
make migrate                  # Run DB migrations
make firewall                 # Build+start firewall container (network isolation)
make cli ARGS="<cmd>"         # Run CLI
make dev-tui-list             # List all TUI dev tools
```

## Notes

- **Temporal must be running** before `make run` — no embedded mode
- **Firewall container** (`noldarim-net-container`) restricts agent network to allowlisted domains (GitHub, PyPI, Go proxy, Anthropic). Silent failure if not running.
- **No CI/CD**: No GitHub Actions — all testing is manual via Makefile
- **SQLite default**: Production should use Postgres (configurable in `config.yaml`)
- **Claude credentials**: Expects `~/.claude.json` on host — copied into agent containers
- **Empty `frontend/` dir**: Legacy artifact, only contains stale `node_modules`. Use `desktop/` instead.
- **Worktrees in `.worktrees/`**: Non-standard location (not `.git/worktrees`); managed by `GitServiceManager`
