ENV_FILE ?= .env

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' $(ENV_FILE))
endif

TAG ?= 0.0.1
BACKEND_PORT ?= 8010
WEB_PORT ?= 3000
AGENT_UID ?= 1001
AGENT_GID ?= 1001
HOST_SUDO ?= sudo
COMPOSE_CMD ?= docker compose --env-file $(ENV_FILE) -f deploy/docker-compose.yml
PRIVATE_SDK_MODULE ?= github.com/nexus-research-lab/nexus-agent-sdk-go

# Default target
.DEFAULT_GOAL := help

.PHONY: help build build-backend build-web start stop restart logs logs-all logs-nginx clean status \
	dev install db-init gen-protocol-types lint-web typecheck-web prepare-host-data \
	check-private-sdk-access check-backend check-go check test run-web run-backend run-backend-go \
	up down log reboot

# Show help
help: ## Show this help message
	@echo "Nexus Core - Available commands:"
	@echo ""
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development commands
run-web: ## Run frontend in development mode
	cd web && pnpm exec vite -- --host 0.0.0.0 --port $(WEB_PORT)

check-private-sdk-access: ## Check private Go SDK access
	@if command -v go >/dev/null 2>&1; then \
		if grep -q "^replace $(PRIVATE_SDK_MODULE) => /" go.mod; then \
			echo "Error: go.mod still contains a local replace for $(PRIVATE_SDK_MODULE)."; \
			echo "The current main branch expects direct access to the private SDK repository."; \
			echo "Remove the local replace first, then follow README.md -> Private Go SDK dependency."; \
			exit 1; \
		fi; \
		effective_goprivate="$${GOPRIVATE:-}"; \
		if [ -z "$$effective_goprivate" ]; then \
			effective_goprivate="$$(go env GOPRIVATE 2>/dev/null || true)"; \
		fi; \
		printf '%s\n' "$$effective_goprivate" | tr ',' '\n' | grep -Fxq "github.com/nexus-research-lab/*" || { \
			echo "Error: GOPRIVATE is not configured for github.com/nexus-research-lab/*."; \
			echo "Set GOPRIVATE and GONOSUMDB before running Go checks or backend commands:"; \
			echo "  go env -w GOPRIVATE=github.com/nexus-research-lab/*"; \
			echo "  go env -w GONOSUMDB=github.com/nexus-research-lab/*"; \
			echo "See README.md -> Private Go SDK dependency for details."; \
			exit 1; \
		}; \
		if ! GIT_TERMINAL_PROMPT=0 go list -m $(PRIVATE_SDK_MODULE) >/dev/null 2>&1; then \
			echo "Error: cannot access private module $(PRIVATE_SDK_MODULE) non-interactively."; \
			echo "Configure SSH or PAT access for github.com/nexus-research-lab/* before running Go checks or backend commands."; \
			echo ""; \
			echo "Recommended GitHub setup:"; \
			echo "  go env -w GOPRIVATE=github.com/nexus-research-lab/*"; \
			echo "  go env -w GONOSUMDB=github.com/nexus-research-lab/*"; \
			echo "  git config --global url.\"git@github.com:\".insteadOf https://github.com/"; \
			echo "  ssh -T git@github.com"; \
			echo ""; \
			echo "If a failed HTTPS checkout was cached, clear it and retry:"; \
			echo "  go clean -modcache"; \
			echo "See README.md -> Private Go SDK dependency for PAT/SSH examples."; \
			exit 1; \
		fi; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi

db-init: check-private-sdk-access ## Run Goose migrations for local database
	go run ./cmd/nexus-migrate up

gen-protocol-types: check-private-sdk-access ## Generate frontend protocol types from Go protocol definitions
	go run ./cmd/protocol-tsgen

run-backend: db-init ## Run Go backend in development mode
	PORT=$(BACKEND_PORT) go run ./cmd/nexus-server

run-backend-go: run-backend ## Alias of run-backend

dev: ## Run both frontend and backend in development mode
	@echo "Starting development servers..."
	@echo "Backend: http://localhost:$(BACKEND_PORT)"
	@echo "Frontend: http://localhost:$(WEB_PORT)"
	@echo "Press Ctrl+C to stop"
	@if lsof -nP -iTCP:$(BACKEND_PORT) -sTCP:LISTEN >/dev/null 2>&1; then \
		echo ""; \
		echo "Error: backend port $(BACKEND_PORT) is already in use."; \
		echo "Hint: stop the existing process or run 'BACKEND_PORT=<port> make dev'."; \
		lsof -nP -iTCP:$(BACKEND_PORT) -sTCP:LISTEN; \
		exit 1; \
	fi
	@if lsof -nP -iTCP:$(WEB_PORT) -sTCP:LISTEN >/dev/null 2>&1; then \
		echo "Warning: frontend port $(WEB_PORT) is already in use, Vite will choose another available port."; \
	fi
	@make -j2 run-web run-backend BACKEND_PORT=$(BACKEND_PORT) WEB_PORT=$(WEB_PORT)

install: check-private-sdk-access ## Install all dependencies
	@echo "Installing Go dependencies..."
	@if command -v go >/dev/null 2>&1; then \
		if ! GIT_TERMINAL_PROMPT=0 go mod tidy; then \
			echo ""; \
			echo "Error: go mod tidy failed while resolving private module $(PRIVATE_SDK_MODULE)."; \
			echo "Most failures here mean git still cannot access github.com/nexus-research-lab/ non-interactively."; \
			echo ""; \
			echo "Recommended GitHub setup:"; \
			echo "  go env -w GOPRIVATE=github.com/nexus-research-lab/*"; \
			echo "  go env -w GONOSUMDB=github.com/nexus-research-lab/*"; \
			echo "  git config --global url.\"git@github.com:\".insteadOf https://github.com/"; \
			echo "  ssh -T git@github.com"; \
			echo ""; \
			echo "If you already ran with a wrong HTTPS config, clear the cached VCS checkout and retry:"; \
			echo "  go clean -modcache"; \
			echo "  go mod tidy"; \
			echo ""; \
			echo "See README.md -> Private Go SDK dependency for PAT/SSH examples."; \
			exit 1; \
		fi; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi
	@echo "Installing frontend dependencies..."
	cd web && pnpm install

lint-web: ## Run frontend lint
	cd web && pnpm run lint

typecheck-web: ## Run frontend type check
	cd web && pnpm run typecheck

check-go: check-private-sdk-access ## Run Go build and test checks
	go test ./...

check-backend: check-go ## Alias of Go backend checks

check: check-go lint-web typecheck-web ## Run basic validation checks

test: check ## Alias of check

# Docker commands
build: ## Build Docker images
	TAG=$(TAG) $(COMPOSE_CMD) build

prepare-host-data: ## Prepare host bind-mount directories for Docker runtime
	@set -eu; \
	host_data_dir="$(HOST_DATA_DIR)"; \
	if [ -z "$$host_data_dir" ]; then \
		host_data_dir="./data"; \
	fi; \
	case "$$host_data_dir" in \
		/*) resolved_dir="$$host_data_dir" ;; \
		~|~/*) resolved_dir="$${HOME}$${host_data_dir#\~}" ;; \
		*) resolved_dir="$(CURDIR)/deploy/$${host_data_dir#./}" ;; \
	esac; \
	echo "Preparing host data directory: $$resolved_dir"; \
	$(HOST_SUDO) mkdir -p "$$resolved_dir" "$$resolved_dir/.nexus" "$$resolved_dir/.claude"; \
	if $(HOST_SUDO) test -d "$$resolved_dir/.claude.json"; then \
		echo "Error: $$resolved_dir/.claude.json is a directory, expected a file."; \
		exit 1; \
	fi; \
	$(HOST_SUDO) touch "$$resolved_dir/.claude.json"; \
	$(HOST_SUDO) chown -R $(AGENT_UID):$(AGENT_GID) "$$resolved_dir/.nexus" "$$resolved_dir/.claude"; \
	$(HOST_SUDO) chown $(AGENT_UID):$(AGENT_GID) "$$resolved_dir/.claude.json"; \
	$(HOST_SUDO) chmod 0755 "$$resolved_dir/.nexus" "$$resolved_dir/.claude"; \
	$(HOST_SUDO) chmod 0644 "$$resolved_dir/.claude.json"; \
	echo "Host data directory is ready: $$resolved_dir"

build-backend: ## Build backend Docker image
	docker build --progress=plain -f deploy/Dockerfile -t leemysw/nexus:app-$(TAG) .

build-web: ## Build frontend + nginx gateway image
	docker build --progress=plain -f web/Dockerfile -t leemysw/nexus:web-$(TAG) .

start: prepare-host-data ## Start all services with Docker
	TAG=$(TAG) $(COMPOSE_CMD) up -d --build --force-recreate
	@echo ""
	@echo "✅ Nexus Core is running!"
	@echo "🌐 Web UI: http://localhost"
	@echo "📋 Backend logs: run 'make logs'"
	@echo "📋 All service logs: run 'make logs-all'"

stop: ## Stop all Docker services
	TAG=$(TAG) $(COMPOSE_CMD) down

restart: stop start ## Restart all Docker services

logs: ## Show backend Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f nexus

logs-all: ## Show all Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f

logs-nginx: ## Show nginx Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f nginx

status: ## Show Docker service status
	TAG=$(TAG) $(COMPOSE_CMD) ps

clean: ## Clean up Docker resources
	TAG=$(TAG) $(COMPOSE_CMD) down -v
	docker system prune -f

# Legacy commands (for backward compatibility)
up: start ## Legacy alias for start
down: stop ## Legacy alias for stop
log: logs ## Legacy alias for logs
reboot: restart ## Legacy alias for restart
