# Orchestrator Package

## OVERVIEW
Core business logic layer. Coordinates data, git, and pipeline services.
Thin coordinator owning runtime dependencies and routing commands.

## STRUCTURE
- `orchestrator.go`: Thin coordinator. Owns Data, Git, and Temporal deps.
- `orchestrator_test.go`: Command routing and integration tests.
- `services/`: Core business logic implementations.
    - `pipeline_service.go`: Pipeline orchestration (700+ lines).
    - `data_service.go`: CRUD for projects, tasks, and runs.
    - `git_service.go`: Low-level git operations (1900+ lines).
    - `git_service_manager.go`: Thread-safe repo pooling.
    - `interfaces.go`: Service interfaces (TemporalClient, etc.).
- `agents/`: AI agent adapters and configuration.
    - `factory.go`: Adapter factory (Claude, test).
    - `claude_adapter.go`: Claude CLI integration.
    - `summary_parser.go`: Structured output extraction.
- `database/`: GORM persistence layer and test fixtures.
- `models/`: Domain models and GORM schema definitions.
    - `gorm_models.go`: Primary schema (Project, Task, PipelineRun).
    - `pipeline.go`: Pipeline domain types.
    - `ai_activity.go`: AI activity record types.
- `temporal/`: Temporal workflows and activities. See `temporal/AGENTS.md`.

## WHERE TO LOOK
- Pipeline orchestration: `services/pipeline_service.go` (start, cancel, fork).
- CRUD/Persistence: `services/data_service.go` (wraps GORM).
- Git operations: `services/git_service.go` (commits, diffs, worktrees).
- Thread-safe Git access: `services/git_service_manager.go` (mutex-based pooling).
- GORM Models: `models/gorm_models.go` (auto-migrated schema).
- Agent adapters: `agents/factory.go` (Claude, test).
- Summary parsing: `agents/summary_parser.go` (extracts JSON markers).
- Test utilities: `services/test_helpers.go` and `mocks_test.go`.

## CONVENTIONS
- Server-first: REST handlers call services directly. Orchestrator loop is legacy.
- Git handles: Use `manager.GetService(path)`, `defer handle.Release()` immediately.
- Tasks: Single-step pipelines. Created via `PipelineService.CreateTask`.
- Service methods: Use params structs (e.g., `CreateTaskParams`) over positional args.
- Summary format: JSON between `---SUMMARY---` and `---END SUMMARY---` markers.
- Persistence: `DataService` wraps GORM for all database operations.
- Interfaces: `services/interfaces.go` defines `TemporalClient` for mocking.
- GORM Tags: Models use GORM tags. Changes auto-migrate on next startup.

## ANTI-PATTERNS
- Using `createTestGitService()`: Deprecated. Use `testutil.WithGitService()`.
- Leaking Git handles: Always `Release()` handles from `GitServiceManager`.
- Positional args: Avoid in `PipelineService` methods. Use params structs.
- Bypassing snapshots: Pipeline forks must use `RunStepSnapshot` for mutations.
- Direct GORM calls: Prefer `DataService` methods for persistence.
- Path traversal: Ensure git operations respect security boundaries.
