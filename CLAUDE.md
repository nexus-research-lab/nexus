# Repository Guidelines

## Project Structure & Module Organization
`agent/` contains the FastAPI backend, WebSocket flow, session management, and persistence (`service/`, `api/`, `shared/`, `utils/`).  
`web/` is the Next.js frontend (`src/components`, `src/hooks`, `src/lib`, `src/store`).  
`alembic/` stores database migrations; keep schema changes and migration files in the same PR.  
`tests/` currently holds integration-style scripts and runtime logs (for example, interrupt/session behavior).  
`docs/` includes architecture and feature guides; update docs when behavior or API contracts change.

## Build, Test, and Development Commands
- `make install`: install backend (`agent/requirements.txt`) and frontend dependencies.
- `make dev`: run backend (`python main.py`) and frontend (`cd web && npm run dev`) together.
- `make run-backend` / `make run-web`: run one side only.
- `alembic upgrade head`: apply latest DB migrations.
- `cd web && npm run build`: production frontend build.
- `cd web && npm run lint`: frontend lint check.
- `python -m py_compile agent/...`: fast backend syntax smoke check.
- `cd web && npx tsc --noEmit`: TypeScript type check.

## Coding Style & Naming Conventions
Python: follow Google Python style, object-oriented design, and use Chinese comments for non-trivial logic. Use `snake_case` for functions/files and `PascalCase` for classes.  
TypeScript/React: components in `PascalCase`, hooks in `useXxx` style, and keep message/session types centralized under `web/src/types`.  
Prefer small, composable handlers/processors over monolithic functions.

## Testing Guidelines
This repository mainly uses integration-style validation instead of heavy unit coverage.  
Primary checks before PR:
- backend compile smoke check (`py_compile`);
- frontend type/lint checks (`tsc`, `eslint`);
- targeted manual flow test for chat/interrupt/session history.  
Add new tests under `tests/` with `test_<feature>.py` naming.

## Commit & Pull Request Guidelines
History favors emoji-prefixed Conventional-style subjects (for example, `:sparkles:`, `:bug:`, `:recycle:`, `:memo:`) with concise Chinese summaries.  
One complete requirement should be delivered as one commit; avoid mixing unrelated changes in the same commit.  
When behavior, API contract, or collaboration rules change, update `CHANGELOG.md` in the same delivery if the change is user-visible or affects follow-up work.  
PRs should include:
- problem statement and scope;
- key file/path changes;
- commands run for validation;
- screenshots for UI changes and sample logs for message/session pipeline updates;
- migration/environment notes when applicable.

## Security & Configuration Tips
Copy env templates (`env.example`, `web/env.example`) and keep secrets local. Never commit API keys, local `.env` files, or debug logs containing sensitive data.
