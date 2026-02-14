# Makefile for noldarim (server-first runtime)

.PHONY: run run-server run-tui build build-server build-tui build-cli migrate cli \
	test test-tui firewall dev-process-task dev-process-task-auto-input dev-create-task \
	build-agent dev-tui dev-tui-commitgraph dev-tui-taskstatus dev-tui-layout \
	dev-tui-layout-lipgloss dev-tui-tokendisplay dev-tui-elapsedtimer dev-tui-stepprogress \
	dev-tui-activityfeed dev-tui-pipelinesummary dev-tui-projectlist dev-tui-taskview \
	dev-tui-settings dev-tui-projectcreation dev-tui-taskdetails dev-adapter \
	dev-dbexplorer dev-dbexplorer-list dev-obsharness dev-tui-list

# Run the primary application runtime (server)
run: run-server

run-server:
	@echo "Running noldarim API server (primary runtime)..."
	go run ./cmd/server

# Run the legacy TUI runtime (deprecated as primary entry path)
run-tui:
	@echo "Running noldarim TUI application (legacy)..."
	go run ./cmd/app

# Build the primary server binary
build: build-server

build-server:
	@echo "Building noldarim API server..."
	go build -o bin/noldarim-server ./cmd/server

# Build the legacy TUI binary
build-tui:
	@echo "Building noldarim TUI application (legacy)..."
	go build -o bin/noldarim-tui ./cmd/app

# Build the CLI application
build-cli:
	@echo "Building noldarim CLI..."
	go build -o bin/noldarim ./cmd/noldarim

# Run database migrations
migrate:
	@echo "Running database migrations..."
	go run ./cmd/migrate

# Run the CLI
cli:
	@go run ./cmd/noldarim $(ARGS)

# Run all tests
test:
	@echo "Running all tests..."
	@if [ "$(shell uname)" = "Darwin" ] && [ -S "$(HOME)/.docker/run/docker.sock" ]; then \
		echo "Detected macOS with Docker Desktop, setting DOCKER_HOST..."; \
		DOCKER_HOST=unix://$(HOME)/.docker/run/docker.sock go test ./...; \
	else \
		go test ./...; \
	fi

# Run TUI package tests only (legacy UI)
test-tui:
	@echo "Running TUI package tests (legacy UI)..."
	go test ./internal/tui/...

# Build and run firewall container
firewall:
	@echo "Building firewall Docker image..."
	docker build -t noldarim-firewall -f docker/go-environment/firewall.Dockerfile docker/go-environment/
	@echo "Stopping existing firewall container if present..."
	docker stop noldarim-net-container 2>/dev/null || true
	docker rm noldarim-net-container 2>/dev/null || true
	@echo "Starting firewall container with NET_ADMIN capability..."
	docker run -d \
		--name noldarim-net-container \
		--cap-add NET_ADMIN \
		--restart unless-stopped \
		noldarim-firewall \
		sh -c "/usr/local/bin/init-firewall.sh 2>&1 | tee /tmp/firewall.log; echo 'Firewall script exit code: '$$?; tail -f /dev/null"
	@echo "Firewall container started successfully"
	@echo "To check firewall logs: docker exec noldarim-net-container cat /tmp/firewall.log"

# Dev tool: Run ProcessTask workflow for development/testing
# Usage: make dev-process-task [TASK_ID=<id>] [PROJECT_ID=<id>] [WORKSPACE_DIR=<dir>]
# If TASK_ID is not provided, the tool auto-selects the latest task
dev-process-task:
	@echo "Running ProcessTask workflow for development..."
	@go run ./cmd/dev/processtask \
		$(if $(TASK_ID),--task-id="$(TASK_ID)") \
		$(if $(PROJECT_ID),--project-id="$(PROJECT_ID)") \
		$(if $(WORKSPACE_DIR),--workspace-dir="$(WORKSPACE_DIR)")

# Dev tool: Auto-run ProcessTask workflow for latest task (legacy helper script)
# Usage: make dev-process-task-auto-input
dev-process-task-auto-input:
	@./dev_get_latest_task.sh

# Dev tool: Create and process a task through orchestrator events
# Usage: make dev-create-task [PROJECT_ID=<id>] [TITLE=<title>] [DESCRIPTION=<desc>] [TOOL=claude|test]
# If PROJECT_ID is not provided, uses the latest project
dev-create-task:
	@echo "Creating and processing task..."
	@go run ./cmd/dev/createtask \
		$(if $(PROJECT_ID),--project-id="$(PROJECT_ID)",--latest-project) \
		$(if $(TITLE),--title="$(TITLE)") \
		$(if $(DESCRIPTION),--description="$(DESCRIPTION)") \
		$(if $(TOOL),--tool="$(TOOL)") \
		$(if $(PROMPT),--prompt="$(PROMPT)") \
		$(if $(TIMEOUT),--timeout="$(TIMEOUT)")

# Build agent Docker image
build-agent:
	@echo "Building agent Docker image..."
	docker build -t noldarim-agent -f docker/go-environment/Dockerfile .

# ========== Legacy TUI Development Commands ==========
# Run specific TUI component demo
# Usage: make dev-tui COMPONENT=<component-name>
dev-tui:
	@if [ -z "$(COMPONENT)" ]; then \
		echo "Error: COMPONENT is required"; \
		echo "Usage: make dev-tui COMPONENT=<component-name>"; \
		echo "Available components:"; \
		echo "  - commitgraph      : Git commit graph visualization"; \
		echo "  - taskstatus       : Task status component states"; \
		echo "  - layout           : Layout wrapper testing"; \
		echo "  - layout_lipgloss  : Alternative layout rendering"; \
		echo "  - tokendisplay     : Token usage display"; \
		echo "  - elapsedtimer     : Elapsed time timer"; \
		echo "  - stepprogress     : Pipeline step progress bar"; \
		echo "  - activityfeed     : AI activity feed"; \
		echo "  - pipelinesummary  : Pipeline run summary box"; \
		echo "  - projectlist      : Project list screen"; \
		echo "  - taskview         : Task view with tabs"; \
		echo "  - settings         : Settings screen"; \
		echo "  - projectcreation  : Project creation form"; \
		echo "  - taskdetails      : Task details view"; \
		exit 1; \
	fi
	@echo "Running TUI $(COMPONENT) demo..."
	@go run ./cmd/dev/tui/$(COMPONENT)

# Convenience targets for TUI components
dev-tui-commitgraph:
	@echo "Running commit graph demo..."
	@go run ./cmd/dev/tui/commitgraph

dev-tui-taskstatus:
	@echo "Running task status demo..."
	@go run ./cmd/dev/tui/taskstatus

dev-tui-layout:
	@echo "Running layout demo..."
	@go run ./cmd/dev/tui/layout

dev-tui-layout-lipgloss:
	@echo "Running layout_lipgloss demo..."
	@go run ./cmd/dev/tui/layout_lipgloss

dev-tui-tokendisplay:
	@echo "Running token display demo..."
	@go run ./cmd/dev/tui/tokendisplay

dev-tui-elapsedtimer:
	@echo "Running elapsed timer demo..."
	@go run ./cmd/dev/tui/elapsedtimer

dev-tui-stepprogress:
	@echo "Running step progress demo..."
	@go run ./cmd/dev/tui/stepprogress

dev-tui-activityfeed:
	@echo "Running activity feed demo..."
	@go run ./cmd/dev/tui/activityfeed

dev-tui-pipelinesummary:
	@echo "Running pipeline summary demo..."
	@go run ./cmd/dev/tui/pipelinesummary

# Convenience targets for TUI screens
dev-tui-projectlist:
	@echo "Running project list screen demo..."
	@go run ./cmd/dev/tui/projectlist

dev-tui-taskview:
	@echo "Running task view screen demo..."
	@go run ./cmd/dev/tui/taskview

dev-tui-settings:
	@echo "Running settings screen demo..."
	@go run ./cmd/dev/tui/settings

dev-tui-projectcreation:
	@echo "Running project creation screen demo..."
	@go run ./cmd/dev/tui/projectcreation

dev-tui-taskdetails:
	@echo "Running task details screen demo..."
	@go run ./cmd/dev/tui/taskdetails

# ========== Adapter Development Commands ==========
# Run adapter on a transcript file
# Usage: make dev-adapter [FILE=<path>] [LINE=<n>] [TYPE=<type>] [RAW=1] [FROM=<n>] [TO=<n>]
# If FILE is not provided, uses the most recent transcript from ~/.claude/projects
dev-adapter:
	@if [ -n "$(FILE)" ]; then \
		echo "Parsing transcript: $(FILE)"; \
		go run ./cmd/dev/adapter \
			$(if $(RAW),--raw) \
			$(if $(LINE),--line $(LINE)) \
			$(if $(TYPE),--type $(TYPE)) \
			$(if $(FROM),--from $(FROM)) \
			$(if $(TO),--to $(TO)) \
			"$(FILE)"; \
	else \
		LATEST_DIR=$$(ls -td ~/.claude/projects/*/ 2>/dev/null | head -1); \
		if [ -z "$$LATEST_DIR" ]; then \
			echo "Error: No directories found in ~/.claude/projects"; \
			exit 1; \
		fi; \
		LATEST_FILE=$$(ls -t "$$LATEST_DIR"*.jsonl 2>/dev/null | head -1); \
		if [ -z "$$LATEST_FILE" ]; then \
			echo "Error: No .jsonl files found in $$LATEST_DIR"; \
			exit 1; \
		fi; \
		echo "Parsing latest transcript: $$LATEST_FILE"; \
		go run ./cmd/dev/adapter \
			$(if $(RAW),--raw) \
			$(if $(LINE),--line $(LINE)) \
			$(if $(TYPE),--type $(TYPE)) \
			$(if $(FROM),--from $(FROM)) \
			$(if $(TO),--to $(TO)) \
			"$$LATEST_FILE"; \
	fi

# ========== Database Explorer Commands ==========
# Explore AI activity events in the database
# Usage: make dev-dbexplorer [TASK_ID=<id>] [TYPE=<event_type>] [LIMIT=<n>] [RAW=1]
# If TASK_ID is not provided, uses the latest task
dev-dbexplorer:
	@go run ./cmd/dev/dbexplorer \
		$(if $(TASK_ID),--task-id="$(TASK_ID)",--latest) \
		$(if $(TYPE),--type="$(TYPE)") \
		$(if $(LIMIT),--limit=$(LIMIT)) \
		$(if $(RAW),--raw)

# List all tasks with AI activity events
dev-dbexplorer-list:
	@go run ./cmd/dev/dbexplorer --list-tasks

# ========== Observability Harness Commands ==========
# Run the observability harness to test AI event pipeline
# Usage: make dev-obsharness WATCH=<dir> [TASK_ID=<id>] [NO_SAVE=1] [RAW=1] [VERBOSE=1]
dev-obsharness:
	@if [ -z "$(WATCH)" ] && [ -z "$(FILE)" ]; then \
		echo "Usage: make dev-obsharness WATCH=<dir> or FILE=<file>"; \
		echo ""; \
		echo "Watch mode: monitors directory for transcript files"; \
		echo "  make dev-obsharness WATCH=./test_transcripts/"; \
		echo ""; \
		echo "File mode: processes a single transcript file"; \
		echo "  make dev-obsharness FILE=./transcript.jsonl"; \
		echo ""; \
		echo "Options:"; \
		echo "  TASK_ID=<id>  - Task ID for saved events (default: dev-harness)"; \
		echo "  NO_SAVE=1     - Don't save to database"; \
		echo "  RAW=1         - Show raw JSON payload"; \
		echo "  VERBOSE=1     - Verbose output"; \
		exit 1; \
	fi
	@go run ./cmd/dev/obsharness \
		$(if $(WATCH),--watch="$(WATCH)") \
		$(if $(FILE),--file="$(FILE)") \
		$(if $(TASK_ID),--task-id="$(TASK_ID)") \
		$(if $(NO_SAVE),--no-save) \
		$(if $(RAW),--raw) \
		$(if $(VERBOSE),--verbose)

# List all available TUI demos
dev-tui-list:
	@echo "Available legacy TUI development commands:"
	@echo ""
	@echo "Components:"
	@echo "  make dev-tui-commitgraph      - Git commit graph visualization"
	@echo "  make dev-tui-taskstatus       - Task status component states"
	@echo "  make dev-tui-layout           - Layout wrapper testing"
	@echo "  make dev-tui-layout-lipgloss  - Alternative layout rendering"
	@echo ""
	@echo "Pipeline Components:"
	@echo "  make dev-tui-tokendisplay     - Token usage display"
	@echo "  make dev-tui-elapsedtimer     - Elapsed time timer"
	@echo "  make dev-tui-stepprogress     - Pipeline step progress bar"
	@echo "  make dev-tui-activityfeed     - AI activity feed"
	@echo "  make dev-tui-pipelinesummary  - Pipeline run summary box"
	@echo ""
	@echo "Screens:"
	@echo "  make dev-tui-projectlist      - Project list screen"
	@echo "  make dev-tui-taskview         - Task view with tabs"
	@echo "  make dev-tui-settings         - Settings screen"
	@echo "  make dev-tui-projectcreation  - Project creation form"
	@echo "  make dev-tui-taskdetails      - Task details view"
	@echo ""
	@echo "Or use: make dev-tui COMPONENT=<name>"
