# Noldarim Development Guide

**Project:** noldarim  
**Last Updated:** 2026-02-14  
**Primary Runtime:** `desktop -> server` (`cmd/server`)

---

## Table of Contents

1. [Architecture Direction](#architecture-direction)
2. [Prerequisites](#prerequisites)
3. [Initial Setup](#initial-setup)
4. [Running the Application](#running-the-application)
5. [Core Make Targets](#core-make-targets)
6. [API and WebSocket Usage](#api-and-websocket-usage)
7. [Development Workflow](#development-workflow)
8. [Developer Tools](#developer-tools)
9. [Legacy TUI Notes](#legacy-tui-notes)
10. [Troubleshooting](#troubleshooting)

---

## Architecture Direction

- Primary product path is **desktop client -> API server**.
- Start the backend with `cmd/server` (`make run` / `make run-server`).
- The TUI runtime (`cmd/app`, `internal/tui`) is **legacy/deprecated as primary entry path**.
- Pipeline execution is Temporal + Docker based, with the agent running inside task containers.

---

## Prerequisites

### Required

- **Go** 1.24.4+
- **Docker** (Docker Desktop on macOS, Docker Engine on Linux)
- **Git**
- **Temporal server** (local dev or cloud)
- **Make**

### Runtime services used by noldarim

- **SQLite** database (default local file)
- **Temporal** workflow orchestration
- **Docker** for running `noldarim-agent` containers

---

## Initial Setup

### 1. Clone and enter repo

```bash
git clone <repository-url>
cd noldarim
```

### 2. Download Go dependencies

```bash
go mod download
```

### 3. Configure

Default config is `config.yaml`. You can keep it as-is for local dev, or copy/override:

```bash
cp config.yaml config.local.yaml
```

Useful sections in `config.yaml`:

- `database`
- `temporal`
- `container`
- `git`
- `server`
- `agent`

### 4. Build agent image (required for pipeline execution)

```bash
make build-agent
```

This builds `noldarim-agent`, which is launched inside task containers.

### 5. Start Temporal

```bash
temporal server start-dev
```

---

## Running the Application

### Primary runtime (server)

```bash
make run
# or
make run-server
```

This starts `cmd/server` (REST + WebSocket API + orchestrator runtime).

### Optional legacy runtime (TUI)

```bash
make run-tui
```

Use only for legacy UI testing; not the primary product path.

### Build binaries

```bash
make build         # bin/noldarim-server
make build-server  # bin/noldarim-server
make build-tui     # bin/noldarim-tui (legacy)
make build-cli     # bin/noldarim
```

---

## Core Make Targets

```bash
make run               # primary: run API server
make run-server        # same as run
make run-tui           # legacy TUI runtime

make build             # build server binary
make build-server
make build-tui         # legacy
make build-cli

make test              # all tests
make test-tui          # legacy TUI package tests

make build-agent       # build Docker image used by task execution
make firewall          # build/run firewall helper container

make cli ARGS="projects"
```

---

## API and WebSocket Usage

Server defaults to `http://localhost:8080`.

### Quick API checks

```bash
curl http://localhost:8080/api/v1/projects
curl http://localhost:8080/api/v1/projects/<project-id>/tasks
curl http://localhost:8080/api/v1/projects/<project-id>/pipelines
```

### Start a pipeline

```bash
curl -X POST http://localhost:8080/api/v1/projects/<project-id>/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Example pipeline",
    "steps": [
      {
        "step_id": "1",
        "name": "Execute task",
        "agent_config": {
          "tool_name": "claude",
          "prompt_template": "Implement feature X",
          "variables": {},
          "tool_options": {"model": "claude-sonnet-4-5"},
          "flag_format": "space"
        }
      }
    ]
  }'
```

### WebSocket stream

Connect to:

- `ws://localhost:8080/ws`

Then send subscription messages:

```json
{"type":"subscribe","filters":{"project_id":"<project-id>"}}
```

---

## Development Workflow

1. Start Temporal: `temporal server start-dev`
2. Build/update agent image when needed: `make build-agent`
3. Run server: `make run`
4. Drive flows from desktop client (primary) or API calls.
5. Run tests: `make test`

For architecture details, see `docs/architecture.md`.

---

## Developer Tools

### Process task workflow tool

```bash
make dev-process-task TASK_ID=<id> PROJECT_ID=<id> WORKSPACE_DIR=/workspace
```

If `TASK_ID` is omitted, the tool tries latest task.

### Auto-input helper

```bash
make dev-process-task-auto-input
```

### Create task via orchestrator/event flow

```bash
make dev-create-task PROJECT_ID=<id> TITLE="My task" DESCRIPTION="Do X" TOOL=claude
```

### Database explorer

```bash
make dev-dbexplorer TASK_ID=<id>
make dev-dbexplorer-list
```

### Transcript adapter parser

```bash
make dev-adapter FILE=<path-to-jsonl>
```

### Observability harness

```bash
make dev-obsharness WATCH=./test_transcripts/
# or
make dev-obsharness FILE=./transcript.jsonl
```

---

## Legacy TUI Notes

Legacy TUI component demos remain available for UI development and regression checks:

```bash
make dev-tui-list
make dev-tui COMPONENT=taskview
make dev-tui-taskview
make dev-tui-settings
make dev-tui-layout-lipgloss
```

This path is not the main application runtime.

---

## Troubleshooting

### Docker not reachable (macOS)

If tests fail to connect to Docker, ensure Docker Desktop is running and socket exists:

```bash
ls -la ~/.docker/run/docker.sock
```

### Temporal connection errors

Make sure Temporal is running and host/namespace match config:

```bash
temporal server start-dev
```

Default expected endpoint: `localhost:7233`.

### Pipeline starts but no container execution

- Confirm `noldarim-agent` image exists: `docker images | grep noldarim-agent`
- Rebuild if needed: `make build-agent`
- Check server logs for container creation / worker queue errors.

### API reachable but no live updates

- Verify WebSocket connection to `/ws`.
- Confirm client sent a valid `subscribe` message (or no filters for all events).
