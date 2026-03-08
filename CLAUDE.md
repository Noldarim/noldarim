# Noldarim

AI agent orchestration platform. Manages isolated container execution of AI-powered development tasks.

## Build & Test

```bash
go build ./...              # Build all packages
make test                   # Run tests (NOT go test — sets Docker socket correctly on macOS)
make run                    # Run server locally
make migrate                # Run database migrations
```

## Project Structure

- `cmd/server/` — API server entry point
- `cmd/agent/` — Agent binary (runs inside containers)
- `cmd/migrate/` — Database migration tool
- `internal/orchestrator/` — Core orchestration logic + Temporal workflows
- `internal/server/` — HTTP API + WebSocket server
- `internal/config/` — Configuration loading (viper)
- `pkg/runtime/` — Runtime provider abstraction (local, sysbox, firecracker)
- `pkg/containers/` — Container backend (Docker API wrapper)
- `desktop/src/` — React 19 + TypeScript + Vite + Tauri 2 frontend
- `sandbox/` — Docker Compose dev sandbox (Sysbox + Temporal + Postgres)

## Architecture

Server-first: Desktop client calls HTTP + WebSocket APIs on `cmd/server`. Execution flows through `internal/orchestrator` → Temporal workflows → Docker containers.

**Runtime Provider abstraction** (`pkg/runtime/`): Provider → Environment → ContainerBackend chain. LocalProvider uses host Docker directly. SysboxProvider (future) provisions isolated environments with their own Docker daemon.

**Container runtime** (`container.container_runtime` config): Sets the Docker `--runtime` flag on agent containers. Set to `sysbox-runc` for isolated Docker-in-Docker (agents can run Docker commands without accessing the host daemon).

## Code Delivery to Agent Containers

How project source code gets into agent containers depends on the deployment mode:

| Mode | Mechanism | Config |
|------|-----------|--------|
| **Local dev** | Git worktree bind-mounted into container | Default. Worktree created at `git.worktree_base_path/.worktrees/<task-id>` |
| **Sandbox (Sysbox compose)** | Source baked into image at build time | `sandbox/Dockerfile` copies source to `/src/noldarim` |
| **Production (future)** | Git clone inside container | Agent clones from remote repo URL provided in task config |

Local dev mounts a git worktree (not the main repo) so each task works on an isolated branch without affecting the working tree.

## Dev Sandbox (Sysbox)

Full isolated stack for integration testing. Requires Sysbox runtime.

```bash
# macOS: Colima VM with Sysbox
brew install colima docker docker-compose
colima start --cpu 4 --memory 8 --disk 60
colima ssh  # then install sysbox .deb inside VM

# Run the sandbox
cd sandbox && docker compose up --build
# API: http://localhost:8080  |  Temporal UI: http://localhost:8233
```

See `sandbox/README.md` for detailed setup.

## Configuration

Config loaded from (in order): `./config.yaml`, `./config/`, `/etc/noldarim/`, `$HOME/.noldarim`. Environment variables override with `NOLDARIM_` prefix (e.g., `NOLDARIM_CONTAINER_DOCKER_HOST`).

Key container settings:
- `container.default_image` — Docker image for agent containers
- `container.container_runtime` — Docker runtime (empty = default runc, `sysbox-runc` = isolated)
- `container.docker_host` — Docker socket path
- `container.runtime_provider` — Runtime provider (`local` = host Docker, future: `sysbox`, `firecracker`)

## Conventions

- Import grouping: stdlib, external, internal (separated by blank lines)
- Tests: table-driven, use testify
- Error handling: wrap with `fmt.Errorf("context: %w", err)`
- Config: all settings in `internal/config/config.go`, accessed via dependency injection
