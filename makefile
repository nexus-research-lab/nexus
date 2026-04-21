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

.PHONY: help build build-backend build-web start stop restart logs clean status \
	dev install db-init gen-protocol-types lint-web typecheck-web prepare-host-data \
	check-backend check-go check test run-web run-backend run-backend-go \
	up down log reboot

# Show help
help: ## Show this help message
	@echo "Nexus Core - Available commands:"
	@echo ""
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development commands
run-web: ## Run frontend in development mode
	cd web && npm exec vite -- --host 0.0.0.0 --port $(WEB_PORT)

db-init: ## Run Goose migrations for local database
	@if command -v go >/dev/null 2>&1; then \
		go run ./cmd/nexus-migrate up; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi

gen-protocol-types: ## Generate frontend protocol types from Go protocol definitions
	@if command -v go >/dev/null 2>&1; then \
		go run ./cmd/protocol-tsgen; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi

run-backend: db-init ## Run Go backend in development mode
	@if command -v go >/dev/null 2>&1; then \
		PORT=$(BACKEND_PORT) go run ./cmd/nexus-server; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi

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

install: ## Install all dependencies
	@echo "Installing Go dependencies..."
	@if command -v go >/dev/null 2>&1; then \
		if grep -q "^replace $(PRIVATE_SDK_MODULE) => /" go.mod; then \
			echo "Error: go.mod still contains a local replace for $(PRIVATE_SDK_MODULE)."; \
			echo "The current main branch expects access to a private SDK repository and cannot be installed from a clean environment until that replace is removed or the dependency strategy is updated."; \
			echo "See README.md -> Private Go SDK dependency for the required GitHub / GOPRIVATE setup and current limitations."; \
			exit 1; \
		fi; \
		go env GOPRIVATE | tr ',' '\n' | grep -Fxq "github.com/nexus-research-lab/*" || { \
			echo "Error: GOPRIVATE is not configured for github.com/nexus-research-lab/*."; \
			echo "Set GOPRIVATE (and usually GONOSUMDB) before running make install:"; \
			echo "  go env -w GOPRIVATE=github.com/nexus-research-lab/*"; \
			echo "  go env -w GONOSUMDB=github.com/nexus-research-lab/*"; \
			echo "See README.md -> Private Go SDK dependency for details."; \
			exit 1; \
		}; \
		git config --get-regexp '^url\..*github\.com[:/]nexus-research-lab/' >/dev/null 2>&1 || { \
			echo "Error: git is not configured with non-interactive credentials for github.com/nexus-research-lab/."; \
			echo "Configure a PAT or SSH access before running make install."; \
			echo "See README.md -> Private Go SDK dependency for examples."; \
			exit 1; \
		}; \
		go mod download; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi
	@echo "Installing frontend dependencies..."
	cd web && npm install

lint-web: ## Run frontend lint
	cd web && npm run lint

typecheck-web: ## Run frontend type check
	cd web && npx tsc --noEmit

check-go: ## Run Go build and test checks
	@if command -v go >/dev/null 2>&1; then \
		go test ./...; \
	else \
		echo "No usable Go runtime found"; \
		exit 1; \
	fi

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
	mkdir -p "$$resolved_dir/.nexus" "$$resolved_dir/.claude"; \
	if [ -d "$$resolved_dir/.claude.json" ]; then \
		echo "Error: $$resolved_dir/.claude.json is a directory, expected a file."; \
		exit 1; \
	fi; \
	touch "$$resolved_dir/.claude.json"; \
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
	@echo "📋 Logs: run 'make logs' to view service logs"

stop: ## Stop all Docker services
	TAG=$(TAG) $(COMPOSE_CMD) down

restart: stop start ## Restart all Docker services

logs: ## Show Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f

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
