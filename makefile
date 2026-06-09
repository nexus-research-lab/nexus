ENV_FILE ?= .env

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' $(ENV_FILE))
endif

TAG ?= 0.1.19
BACKEND_PORT ?= 8010
WEB_PORT ?= 3000
AGENT_UID ?= 1001
AGENT_GID ?= 1001
HOST_SUDO ?= sudo
APP_WIN_BUILD_NUMBER ?= $(shell pwsh -NoLogo -NoProfile -Command "Get-Date -Format yyyyMMddHHmmss")
APP_WIN_OUTPUT_DIR ?=
APP_WIN_BUNDLE_NXS_RUNTIME ?= 1
APP_WIN_RUN_SKIP_BUILD ?=
APP_WIN_RUN_WAIT ?=
NXS_DEV_GOOS ?= $(shell go env GOOS)
NXS_DEV_GOARCH ?= $(shell go env GOARCH)
NXS_DEV_BINARY_NAME := nxs
ifeq ($(NXS_DEV_GOOS),windows)
NXS_DEV_BINARY_NAME := nxs.exe
endif
NXS_DEV_RUNTIME_PATH ?= $(abspath ../nexus-agent-sdk/nexus-agent-sdk-go/dist/nxs/$(NXS_DEV_GOOS)-$(NXS_DEV_GOARCH)/$(NXS_DEV_BINARY_NAME))
COMPOSE_CMD ?= docker compose --env-file $(ENV_FILE) -f deploy/docker-compose.yml

# Default target
.DEFAULT_GOAL := help

.PHONY: help build build-backend build-web package-release start stop restart logs logs-all logs-nginx clean status \
	dev dev-nxs install gen-protocol-types lint-web typecheck-web prepare-host-data \
	check-backend check-go check test run-web run-backend run-backend-go \
	app-build-dev app-run-dev app-build app-run app-smoke app-package app-dmg build-dmg app-check app-win-build app-win-run app-win-smoke app-win-package \
	pull deploy start-no-build

# Show help
help: ## Show this help message
	@echo "Nexus Core - Available commands:"
	@echo ""
ifeq ($(NXS_DEV_GOOS),windows)
	@pwsh -NoLogo -NoProfile -Command "$$pattern = '^([a-zA-Z_-]+):.*?## (.*)$$'; Get-Content '$(MAKEFILE_LIST)' | ForEach-Object { if ($$_ -match $$pattern) { [pscustomobject]@{ Target = $$Matches[1]; Description = $$Matches[2] } } } | Sort-Object Target | ForEach-Object { '  {0,-20} {1}' -f $$_.Target, $$_.Description }"
else
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
endif

# Development commands
run-web: ## Run frontend in development mode
	cd web && pnpm exec vite -- --host 0.0.0.0 --port $(WEB_PORT)

gen-protocol-types: ## Generate frontend protocol types from Go protocol definitions
	go generate ./internal/protocol

run-backend: ## Run Go backend in development mode
	NEXUS_APP_ROOT=$${NEXUS_APP_ROOT:-$(CURDIR)} PORT=$(BACKEND_PORT) go run ./cmd/nexus-server

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

dev-nxs: ## Run dev servers with local Go SDK nxs runtime
	@if [ -z "$$NEXUS_NXS_COMMAND_PATH" ] && [ ! -x "$(NXS_DEV_RUNTIME_PATH)" ]; then \
		echo "Error: dev nxs runtime not found: $(NXS_DEV_RUNTIME_PATH)"; \
		echo "Hint: run 'make -C ../nexus-agent-sdk/nexus-agent-sdk-go build-nxs' first."; \
		exit 1; \
	fi
	NEXUS_AGENT_RUNTIME_KIND=nxs NEXUS_NXS_COMMAND_PATH="$${NEXUS_NXS_COMMAND_PATH:-$(NXS_DEV_RUNTIME_PATH)}" $(MAKE) dev BACKEND_PORT=$(BACKEND_PORT) WEB_PORT=$(WEB_PORT)

install: ## Install all dependencies
	@echo "Installing Go dependencies..."
	@if command -v go >/dev/null 2>&1; then \
		if ! GIT_TERMINAL_PROMPT=0 go mod tidy; then \
			echo ""; \
			echo "Error: go mod tidy failed."; \
			echo "Resolve the Go module error, then retry:"; \
			echo "  go mod tidy"; \
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

check-go: ## Run Go build and test checks
	go test ./...

check-backend: check-go ## Alias of Go backend checks

check: check-go lint-web typecheck-web ## Run basic validation checks

test: check ## Alias of check

app-build-dev: ## 构建 macOS 桌面开发版 shell
	./scripts/desktop/build-macos-dev.sh

app-run-dev: ## 构建并运行 macOS 桌面开发版 shell
	./scripts/desktop/run-macos-dev.sh

app-build: ## 构建 ad-hoc macOS .app
	./scripts/desktop/build-macos-app.sh

app-run: ## 构建并运行 ad-hoc macOS .app
	./scripts/desktop/run-macos-app.sh

app-smoke: ## 烟测已组装的 macOS .app
	./scripts/desktop/smoke-macos-app.sh

app-package: ## 构建 macOS app zip、sha256 和 metadata
	NEXUS_DESKTOP_PACKAGE_FORMAT=zip ./scripts/desktop/package-macos-app.sh

app-dmg: ## 构建 macOS app dmg、sha256 和 metadata
	NEXUS_DESKTOP_PACKAGE_FORMAT=dmg ./scripts/desktop/package-macos-app.sh

build-dmg: app-dmg ## app-dmg 的别名

app-check: app-smoke ## 构建并烟测 macOS .app

app-win-build: ## 构建 Windows WPF/WebView2 桌面 app
	pwsh scripts/desktop/build-windows-app.ps1 -BuildNumber "$(APP_WIN_BUILD_NUMBER)" -OutputDir "$(APP_WIN_OUTPUT_DIR)" -BundleNXSRuntime "$(APP_WIN_BUNDLE_NXS_RUNTIME)"

app-win-run: ## 构建并运行 Windows WPF/WebView2 桌面 app
	pwsh scripts/desktop/run-windows-app.ps1 -BuildNumber "$(APP_WIN_BUILD_NUMBER)" -OutputDir "$(APP_WIN_OUTPUT_DIR)" -BundleNXSRuntime "$(APP_WIN_BUNDLE_NXS_RUNTIME)" $(if $(filter 1 true yes on,$(APP_WIN_RUN_SKIP_BUILD)),-SkipBuild,) $(if $(filter 1 true yes on,$(APP_WIN_RUN_WAIT)),-Wait,)

app-win-smoke: ## 烟测已组装的 Windows WPF/WebView2 桌面 app
	pwsh scripts/desktop/smoke-windows-app.ps1

app-win-package: ## 构建、烟测并打包 Windows WPF/WebView2 桌面 app installer、sha256 和 metadata
	pwsh scripts/desktop/package-windows-app.ps1

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

package-release: ## Build Go + web release package without macOS app
	./scripts/package-release.sh $(TAG)

start: prepare-host-data ## Start all services with Docker
	TAG=$(TAG) $(COMPOSE_CMD) up -d --build --force-recreate
	@echo ""
	@echo "✅ Nexus Core is running!"
	@echo "🌐 Web UI: http://localhost"
	@echo "📋 Backend logs: run 'make logs'"
	@echo "📋 All service logs: run 'make logs-all'"

start-no-build: prepare-host-data
	TAG=$(TAG) $(COMPOSE_CMD) up -d --force-recreate
	@echo ""
	@echo "✅ Nexus Core is running!"
	@echo "🌐 Web UI: http://localhost"
	@echo "📋 Backend logs: run 'make logs'"
	@echo "📋 All service logs: run 'make logs-all'"

stop: ## Stop all Docker services
	TAG=$(TAG) $(COMPOSE_CMD) down

restart: stop start ## Restart all Docker services

logs: ## Show backend Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f nexus -n 1000

logs-all: ## Show all Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f -n 1000

logs-nginx: ## Show nginx Docker service logs
	TAG=$(TAG) $(COMPOSE_CMD) logs -f nginx -n 1000

status: ## Show Docker service status
	TAG=$(TAG) $(COMPOSE_CMD) ps

clean: ## Clean up Docker resources
	TAG=$(TAG) $(COMPOSE_CMD) down -v
	docker system prune -f


# deploy
pull:
	git pull origin main

deploy:
	$(MAKE) pull
	$(MAKE) build
	$(MAKE) stop
	$(MAKE) start-no-build
