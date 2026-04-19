# Repository Guidelines (backend/)

> 本文件描述 emomo monorepo 的 Go 后端子项目。仓库整体约定见 [../AGENTS.md](../AGENTS.md)；前端约定见 [../frontend/AGENTS.md](../frontend/AGENTS.md)。
> 所有命令默认在 `backend/` 目录下执行（`cd backend`）。

## Project Structure & Module Organization
- `cmd/`: Go entry points (`cmd/api`, `cmd/ingest`).
- `internal/`: Go application code (API handlers, services, repositories, sources, storage).
- `configs/`: YAML config files and examples.
- `migrations/`: SQL migrations.
- `scripts/`: Backend-only helper scripts (`import-data.sh`, `check-data-dir.sh`, `setup.sh`, `clear-qdrant.sh`).
- `data/`: Local data and staging directories (gitignored except for `.gitkeep`).
- `Dockerfile`, `.dockerignore`: Container build definition (also pushed to Hugging Face Space via subtree split).

Sibling directories at repo root: `../frontend/` (React/Vite UI), `../crawler/` (Python crawler), `../deployments/` (cross-service compose), `../docs/`, `../scripts/start.sh`.

## Build, Test, and Development Commands
- `cd backend && go run ./cmd/api`: run the API server locally (port 8080 by default).
- `cd backend && go build ./... && go test ./...`: build and test all Go packages.
- `cd backend && ./scripts/import-data.sh -s staging:fabiaoqing -l 50`: ingest staged memes (recommended).
- `cd backend && go run ./cmd/ingest --source=staging:fabiaoqing --limit=50`: ingest staged memes (alternative).
- `docker compose -f deployments/docker-compose.yml up -d` (from repo root): start API + Grafana Alloy.
- `cd crawler && uv sync && uv run emomo-crawler crawl --source fabiaoqing --limit 100`: populate `backend/data/staging/`.

## Coding Style & Naming Conventions
- Go: follow `gofmt` defaults (tabs for indentation); package names short and lowercase.
- Config: keep new keys grouped by subsystem under `backend/configs/`.
- Logging: prefer the helpers in `internal/logger` (context-aware fields).

## Testing Guidelines
- Go tests: `cd backend && go test ./...`.
- Add table-driven tests for service-layer logic when introducing new behavior.

## Commit & Pull Request Guidelines
- Commit messages follow Conventional Commits (`feat:`, `fix:`, `chore:` ...). Keep the subject short and imperative.
- For changes that touch multiple subprojects, scope by directory in the body (e.g. "backend: ...; frontend: ...").
- PRs should describe scope, link related issues, and include curl examples or screenshots for API-visible changes.

## Security & Configuration Tips
- Never commit API keys or secrets; use `backend/.env` (gitignored) or environment variables.
- For production, prefer TLS-enabled endpoints (`QDRANT_USE_TLS=true`, `STORAGE_USE_SSL=true`).
- The Hugging Face Space deploy mirror only sees `backend/`, so don't introduce paths that escape this directory at runtime.
