TAG:=0.0.1

# Default target
.DEFAULT_GOAL := help

.PHONY: help build start stop restart logs clean status dev install test

# Show help
help: ## Show this help message
	@echo "Nexus Core - Available commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development commands
run-web: ## Run frontend in development mode
	cd web && npm run dev

run-backend: ## Run backend in development mode
	@if [ -x .venv/bin/python ]; then \
		.venv/bin/python main.py; \
	elif command -v python3 >/dev/null 2>&1; then \
		python3 main.py; \
	elif command -v python >/dev/null 2>&1; then \
		python main.py; \
	else \
		echo "No usable Python runtime found"; \
		exit 1; \
	fi

dev: ## Run both frontend and backend in development mode
	@echo "Starting development servers..."
	@echo "Backend: http://localhost:8010"
	@echo "Frontend: http://localhost:3000"
	@echo "Press Ctrl+C to stop"
	@make -j2 run-web run-backend

install: ## Install all dependencies
	@echo "Installing backend dependencies..."
	@if [ -x .venv/bin/python ] && .venv/bin/python -m pip --version >/dev/null 2>&1; then \
		PYTHON=.venv/bin/python; \
		$$PYTHON -m pip install -r agent/requirements.txt; \
	elif command -v uv >/dev/null 2>&1; then \
		uv pip install -r agent/requirements.txt --index-url https://mirrors.aliyun.com/pypi/simple; \
	elif command -v python3 >/dev/null 2>&1 && python3 -m pip --version >/dev/null 2>&1; then \
		PYTHON=$$(command -v python3); \
		$$PYTHON -m pip install -r agent/requirements.txt; \
	else \
		echo "No usable Python package installer found (.venv pip, uv, or python3 -m pip)"; \
		exit 1; \
	fi
	@echo "Installing frontend dependencies..."
	cd web && npm install

# Docker commands
build: ## Build Docker images
	TAG=$(TAG) docker compose -f deploy/docker-compose.yml build

build-backend: ## Build backend Docker image
	docker build --progress=plain -f deploy/Dockerfile -t leemysw/nexus-core:app-$(TAG) .

build-web: ## Build frontend Docker image
	docker build --progress=plain -f web/Dockerfile -t leemysw/nexus-core:web-$(TAG) ./web

start: ## Start all services with Docker
	@if ! docker network inspect net >/dev/null 2>&1; then \
		echo "Creating Docker network 'net'..."; \
		docker network create net; \
	fi
	TAG=$(TAG) docker compose -f deploy/docker-compose.yml up -d
	@echo ""
	@echo "✅ Nexus Core is running!"
	@echo "🌐 Web UI: http://localhost"
	@echo "📋 Logs: run 'make logs' to view service logs"

stop: ## Stop all Docker services
	TAG=$(TAG) docker compose -f deploy/docker-compose.yml down

restart: stop start ## Restart all Docker services

logs: ## Show Docker service logs
	TAG=$(TAG) docker compose -f deploy/docker-compose.yml logs -f

status: ## Show Docker service status
	TAG=$(TAG) docker compose -f deploy/docker-compose.yml ps

clean: ## Clean up Docker resources
	TAG=$(TAG) docker compose -f deploy/docker-compose.yml down -v
	docker system prune -f

# Legacy commands (for backward compatibility)
up: start ## Legacy alias for start
down: stop ## Legacy alias for stop
log: logs ## Legacy alias for logs
reboot: restart ## Legacy alias for restart
