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

**Runtime Provider abstraction** (`pkg/runtime/`): Provider → Environment → ContainerBackend chain.
- `LocalProvider`: uses host Docker directly (default).
- `SysboxProvider`: provisions a Sysbox container with its own Docker daemon; agent containers run inside that isolated daemon. Set `container.runtime_provider: sysbox`.

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
- `container.runtime_provider` — Runtime provider (`local` = host Docker, `sysbox` = isolated Docker daemon)
- `container.sysbox_image` — Docker image for Sysbox environment containers (default: `docker:27-dind`)

## Agent Images

- **`noldarim-agent`** (default) — minimal image with agent binary, ca-certificates, git
- **`noldarim-agent-full`** — Go toolchain + Docker CLI, for use with SysboxProvider
- **`noldarim-agent-sysbox`** — per-agent Sysbox isolation: nestybox base with systemd + Docker daemon + Go toolchain. Each agent container gets its own Docker daemon. Build with: `docker build -t noldarim-agent-sysbox -f build/Dockerfile.agent-sysbox .`

## Dogfooding Setup (Isolated Agent Execution)

Agents run with network firewall isolation via the `noldarim-net-container`:

```bash
# 1. Start firewall container (network isolation for agents)
make firewall

# 2. Start Temporal infrastructure
cd sandbox && docker compose up -d postgres temporal && cd ..

# 3. Build the agent image
docker build -t noldarim-agent-full -f build/Dockerfile.agent-full .

# 4. Run server (agents use default network_mode: container:noldarim-net-container)
NOLDARIM_TEMPORAL_HOST_PORT=localhost:7233 \
NOLDARIM_CONTAINER_DEFAULT_IMAGE=noldarim-agent-full \
make run
```

Agents can run `go test` (unit tests) with network restricted to allowlisted domains.
Monitor blocked traffic: `make firewall-denied`

**Sysbox variant** (for `make test` with Docker-dependent integration tests):
Use `noldarim-agent-sysbox` image + `container_runtime: sysbox-runc`. Requires
`network_mode: ""` (incompatible with shared firewall container — each Sysbox agent
has its own network namespace). See `build/Dockerfile.agent-sysbox`.

## Conventions

- Import grouping: stdlib, external, internal (separated by blank lines)
- Tests: table-driven, use testify
- Error handling: wrap with `fmt.Errorf("context: %w", err)`
- Config: all settings in `internal/config/config.go`, accessed via dependency injection
