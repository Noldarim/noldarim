# Noldarim

## Purpose

Noldarim is a workflow orchestrator for AI-assisted software tasks.  
It runs tasks as pipelines with Temporal orchestration, Git/worktree management, and Docker-based agent execution.

## Use It (Current State)

Primary runtime is **server-first** (`cmd/server`).

1. Start Temporal:
   ```bash
   temporal server start-dev
   ```
2. Build the agent image:
   ```bash
   make build-agent
   ```
3. Run the API server:
   ```bash
   make run
   ```
4. Use the API (default: `http://127.0.0.1:8080`):
   ```bash
   curl -X POST http://127.0.0.1:8080/api/v1/projects \
     -H "Content-Type: application/json" \
     -d '{"name":"Demo","description":"Local repo","repository_path":"/absolute/path/to/repo"}'

   curl http://127.0.0.1:8080/api/v1/projects
   ```

WebSocket events are available at `ws://127.0.0.1:8080/ws`.

## Run for Development

```bash
go mod download
make build-agent
make run
```

Useful commands:

```bash
make test
make build-server
make run-tui   # legacy UI path
```

## Required Dependencies

- Go `1.24.4+`
- Docker
- Temporal server
- Git
- Make
- Claude credentials/config on host (`~/.claude.json`) for default agent flow

## Architecture

- [architecture.md](docs/architecture.md)
- [components-hierarchy.md](components-hierarchy.md)

## License

- Community: [AGPL-3.0-or-later](LICENSE)
- Commercial license: contact `contact@noldarim.com`
