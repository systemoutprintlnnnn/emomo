# CLAUDE.md (backend/)

This file provides guidance to Claude Code (claude.ai/code) when working in the Go backend of the emomo monorepo. For repo-wide context see [../CLAUDE.md](../CLAUDE.md); for the React frontend see [../frontend/CLAUDE.md](../frontend/CLAUDE.md).

All commands below assume `cd backend` unless stated otherwise.

## Project Overview

Emomo is an AI-powered meme/sticker semantic search system. Users can search for memes using natural language queries in Chinese.

**Tech Stack:** Go 1.24 + Gin, Qdrant (vector DB), S3-compatible storage (R2/S3), SQLite/PostgreSQL, OpenAI-compatible VLM (e.g., GPT-4o mini), Jina Embeddings v3, Grafana Alloy + Loki (logging)

## Build & Run Commands

```bash
# Build binaries (optional, can use go run instead)
cd backend
go build -o api ./cmd/api

# Start infrastructure (API + Alloy; Qdrant/S3 are external)
# (run from repo root)
docker compose -f deployments/docker-compose.yml up -d

# Logs only (Grafana Alloy)
docker compose -f deployments/docker-compose.yml up -d alloy

# Data ingestion using script (recommended, no build required)
cd backend
./scripts/import-data.sh -p ./data/memes -l 100     # Ingest memes
./scripts/import-data.sh -r -l 100                  # Retry pending items
./scripts/import-data.sh -p ./data/memes -f         # Force re-process

# Or use go run directly
go run ./cmd/ingest --source=localdir --path=./data/memes --limit=100

# Run API server (port 8080)
go run ./cmd/api

# Full stack (backend + frontend) — from repo root
../scripts/start.sh
```

## Architecture

```
backend/
├── cmd/
│   ├── api/main.go      # REST API server entry point
│   └── ingest/main.go   # Data ingestion CLI tool
├── internal/
│   ├── api/
│   │   ├── router.go    # Route configuration
│   │   └── handler/     # HTTP handlers (search, meme, health)
│   ├── service/
│   │   ├── search.go    # Semantic search (query → embedding → Qdrant)
│   │   ├── ingest.go    # Ingestion pipeline with worker pool
│   │   ├── vlm.go       # OpenAI-compatible VLM client for image descriptions
│   │   └── embedding.go # Text embeddings (multi-model registry)
│   ├── repository/
│   │   ├── meme_repo.go # Relational DB operations
│   │   └── qdrant_repo.go # Vector search operations (gRPC)
│   ├── storage/s3.go    # S3-compatible object storage (supports R2, S3, etc.)
│   ├── source/          # Data source adapters
│   │   └── localdir/    # Local static image directory source
│   ├── logger/          # Context-aware structured logging
│   └── domain/          # Data models (Meme, Source, Job)
└── migrations/          # SQL migrations
```

### Data Flow

1. **Ingestion**: Source adapter → VLM description → text embedding → object storage upload → Qdrant upsert → DB save.
2. **Search**: Query text → (optional) VLM query expansion → text embedding → Qdrant cosine similarity → return top-K results.

## API Endpoints

- `POST /api/v1/search` - Semantic meme search (`{"query": "text", "top_k": 20}`)
- `GET /api/v1/categories` - List categories
- `GET /api/v1/memes` - List memes (supports `category`, `limit`, `offset`)
- `GET /api/v1/memes/{id}` - Get meme details
- `GET /api/v1/stats` - System statistics
- `GET /health` - Health check

## Configuration

Environment variables (see `backend/.env.example`):
- **VLM**: `VLM_MODEL`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`
- **Embeddings**: `JINA_API_KEY`, `MODELSCOPE_API_KEY`, `MODELSCOPE_BASE_URL` (and any other `*_API_KEY` referenced in `configs/config.yaml`)
- **Search**: `QUERY_EXPANSION_MODEL`
- **Storage**: `STORAGE_TYPE`, `STORAGE_ENDPOINT`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_PUBLIC_URL`
- **Qdrant**: `QDRANT_HOST`, `QDRANT_PORT`, `QDRANT_API_KEY`, `QDRANT_USE_TLS`
- **Database**: `DATABASE_DRIVER` (sqlite/postgres), `DATABASE_PATH` or `DATABASE_URL`
- **Monitoring**: `LOKI_URL`, `LOKI_USERNAME`, `LOKI_PASSWORD`, `CLUSTER_NAME`, `ENVIRONMENT`

Config file: `backend/configs/config.yaml`.

## Deployment & Monitoring

- **Compose**: `deployments/docker-compose.yml` for API + Alloy (Qdrant/S3 are external).
- **Render / Railway**: configured via `render.yaml` (`rootDir: backend`) and `railway.json` (`dockerfilePath: backend/Dockerfile`).
- **Hugging Face Space**: GitHub Actions splits `backend/` as a subtree and force-pushes it to the Space's `main`. The Space sees `backend/` contents as its root.
- **Logging**: Grafana Alloy collects Docker container logs and forwards to Grafana Cloud Loki. Alloy UI: `http://localhost:12345`.

## Key Patterns

- **Source Interface**: New data sources implement `Source` interface in `internal/source/`.
- **Worker Pool**: Ingest service uses goroutine workers with configurable concurrency.
- **Layered Architecture**: Handler → Service → Repository → Storage.
- **Meme Status**: `pending` (awaiting VLM) → `active` (ready) or `failed`.
- **Multi-embedding**: each embedding is registered in `internal/service/embedding_registry.go` and stored as a separate vector row.
