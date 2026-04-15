# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Build & Validation Commands
- `make dev` — start backend (port 8010) + frontend (port 3000) concurrently
- `make check` — runs `check-backend` + `lint-web` + `typecheck-web` (the full pre-PR validation suite)
- `make check-backend` — `py_compile` on all `agent/**/*.py` via `rg --files`
- `cd web && npx tsc --noEmit` — TypeScript type check
- `cd web && npm run lint` — ESLint on `src/**/*.{ts,tsx}`
- `make db-init` — run Alembic migrations (auto-detects `.venv` / system Python)
- `make install` — installs backend deps (prefers `.venv`, then `uv`, then `pip`) + `npm install` in `web/`

## Critical Conventions
- **Python file size hard limit: 300 lines**, target 100–200. One class per file. Split proactively.
- **Chinese comments** required for non-trivial logic blocks.
- Frontend naming: in `web/src`, ordinary functions, `interface/type` field names, component custom props, hook return values, and hook option objects should use `snake_case`.
- Frontend naming exceptions: `interface/type/class/component` identifiers stay `PascalCase`; Hooks stay `useXxx`; React/DOM/third-party protocol fields keep their original names (e.g. `className`, `onClick`, `multiSelect`, `keyCode`, `saveData`).
- Frontend protocol rule: for backend request/response fields and SDK/tool payloads, follow the actual upstream contract first; do not rename wire fields only for style consistency.
- All API routes live under prefix `/agent/v1/...` (set in [`config.py`](agent/config/config.py:47)).
- Settings use `pydantic-settings` with `case_sensitive=True` and `extra="allow"` — env vars must match field names exactly.
- Pydantic models must extend [`AModel`](agent/infra/schemas/model_cython.py:23) (not raw `BaseModel`) to handle CyFunction detection.
- API responses use [`resp.ok()`](agent/infra/server/common/base_resp.py:22) / [`resp.fail()`](agent/infra/server/common/base_resp.py:23) pattern from `agent.infra.server.common`.
- Exceptions extend [`ServerException`](agent/infra/server/common/base_exception.py:14) with a `.resp` attribute mapping to HTTP response.
- Use [`@exception_to_base_error`](agent/infra/server/common/base_error_warp.py:21) decorator to auto-wrap service-layer exceptions.
- ID generation uses custom [Snowflake](agent/utils/snowflake.py) (not UUID).

## Architecture Flow
- Entry: [`main.py`](main.py) → [`agent/app.py`](agent/app.py) (FastAPI app with lifespan)
- Lifespan registers message channels (WebSocket, Discord, Telegram) via [`ChannelRegister`](agent/service/channels/channel_register.py:26)
- WebSocket messages routed through [`ChannelDispatcher`](agent/service/channels/ws/dispatcher.py:23) → handler chain (interrupt, permission, ping, error)
- Chat processing: [`ChatService`](agent/service/chat/chat_service.py) → [`ChatMessageProcessor`](agent/service/message/chat_message_processor.py:30) → Claude Agent SDK
- Storage dual-layer: file-based JSON/JSONL in [`agent/storage/`](agent/storage/) + SQLite via SQLAlchemy in [`agent/storage/sqlite/`](agent/storage/sqlite/)
- Frontend: React 19 + Vite 7 + Zustand stores. Path alias `@/` → `web/src/`. WebSocket client in [`lib/websocket/`](web/src/lib/websocket/).

## Commit Style
Emoji-prefixed Conventional commits with Chinese summaries (e.g., `:sparkles: 添加新功能`). Update `CHANGELOG.md` for user-visible changes.
