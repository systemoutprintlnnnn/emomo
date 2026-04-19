# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working in this repository. It covers the monorepo layout; subproject-specific instructions live in each subdirectory.

## Subproject Map

This is a multi-language monorepo. Open the nearest CLAUDE.md to whatever you are touching:

- [backend/CLAUDE.md](backend/CLAUDE.md) — Go backend (REST API + ingestion pipeline + Qdrant/storage integration)
- [frontend/CLAUDE.md](frontend/CLAUDE.md) — React 19 + Vite frontend
- `crawler/` — Python crawler (no dedicated CLAUDE.md yet; see `crawler/README.md`)

## Repo Layout

```
backend/      Go application (cmd/, internal/, configs/, migrations/, Dockerfile)
frontend/    React + Vite SPA
crawler/     Python crawler managed by uv
deployments/ Docker Compose orchestration
docs/        Cross-service docs
scripts/     Cross-service helpers (start.sh)
```

## Common Commands

```bash
# Start backend + frontend for local development
./scripts/start.sh

# Backend build / test
cd backend && go build ./... && go test ./...

# Frontend lint / build
cd frontend && npm install && npm run lint && npm run build

# Crawler
cd crawler && uv sync && uv run emomo-crawler crawl --source fabiaoqing --limit 100

# Containerized API + Grafana Alloy
docker compose -f deployments/docker-compose.yml up -d
```

## Deployment

- Render: configured via [render.yaml](render.yaml) (`rootDir: backend`).
- Railway: [railway.json](railway.json) points at `backend/Dockerfile`.
- Hugging Face Space: [.github/workflows/sync_to_hf.yml](.github/workflows/sync_to_hf.yml) splits `backend/` as a subtree and force-pushes it to the Space's `main`. Anything outside `backend/` does not reach the Space.

## Tips When Editing

- Keep changes scoped to one subproject when possible; cross-cutting commits should clearly call out which directory each hunk belongs to.
- The Go module path is `github.com/timmy/emomo` and is independent of the on-disk path; do not rewrite imports just because files moved.
- Backend runtime expects `cwd = backend/` (so `./configs`, `./data` resolve correctly). Don't introduce code that assumes the repo root is the cwd.
