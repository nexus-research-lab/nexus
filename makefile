TAG:=0.2.0

# Default target
.DEFAULT_GOAL := help

.PHONY: help build start stop restart logs clean status dev install test

# Show help
help: ## Show this help message
	@echo "Nexus Core - Available commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Database commands
init-alembic: ## Initialize Alembic for database migrations
	alembic init -t async alembic

init-db: ## Initialize database structure
	alembic revision --autogenerate -m "初始数据库结构"

upgrade-db: ## Upgrade database to latest version
	alembic upgrade head

downgrade-db: ## Downgrade database to base version
	alembic downgrade base

create-migration: ## Create new migration (usage: make create-migration MSG="description")
	@if [ -z "$(MSG)" ]; then \
		echo "Usage: make create-migration MSG='your message'"; \
		exit 1; \
	fi
	alembic revision --autogenerate -m "$(MSG)"

current-version: ## Show current database version
	alembic current

history-version: ## Show database migration history
	alembic history --verbose

# Development commands
run-web: ## Run frontend in development mode
	cd web && npm run dev

run-backend: ## Run backend in development mode
	python main.py

dev: ## Run both frontend and backend in development mode
	@echo "Starting development servers..."
	@echo "Backend: http://localhost:8010"
	@echo "Frontend: http://localhost:3000"
	@echo "Press Ctrl+C to stop"
	@make -j2 run-web run-backend

install: ## Install all dependencies
	@echo "Installing backend dependencies..."
	pip install -r agent/requirements.txt
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

# Database management
reset-db: ## Reset database (WARNING: This will delete all data!)
	@echo "WARNING: This will delete all database data!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		echo ""; \
		echo "Stopping services..."; \
		TAG=$(TAG) docker compose -f deploy/docker-compose.yml down; \
		echo "Removing database volume..."; \
		docker volume rm Nexus-Core_db_data 2>/dev/null || true; \
		echo "Starting services..."; \
		TAG=$(TAG) docker compose -f deploy/docker-compose.yml up -d; \
		echo "Database reset complete!"; \
	else \
		echo ""; \
		echo "Cancelled."; \
	fi
