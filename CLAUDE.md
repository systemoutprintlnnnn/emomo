# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Emomo is an AI-powered meme/sticker semantic search system. Users can search for memes using natural language queries in Chinese.

**Tech Stack:** Go 1.24 + Gin, Qdrant (vector DB), MinIO (object storage), SQLite/PostgreSQL, OpenAI GPT-4o mini (VLM), Jina Embeddings v3

## Build & Run Commands

```bash
# Build binaries
go build -o api ./cmd/api
go build -o ingest ./cmd/ingest

# Start infrastructure (Qdrant + MinIO)
docker-compose -f deployments/docker-compose.yml up -d

# Data ingestion
./ingest --source=chinesebqb --limit=100    # Ingest memes
./ingest --retry --limit=100                # Retry pending items
./ingest --force --source=chinesebqb        # Force re-process

# Run API server (port 8080)
./api

# Full stack (backend + frontend)
./scripts/start.sh
```

## Architecture

```
cmd/
├── api/main.go          # REST API server entry point
└── ingest/main.go       # Data ingestion CLI tool

internal/
├── api/
│   ├── router.go        # Route configuration
│   └── handler/         # HTTP handlers (search, meme, health)
├── service/
│   ├── search.go        # Semantic search (query → embedding → Qdrant)
│   ├── ingest.go        # Ingestion pipeline with worker pool
│   ├── vlm.go           # GPT-4o mini for image descriptions
│   └── embedding.go     # Jina text embeddings (1024-dim)
├── repository/
│   ├── meme_repo.go     # SQLite/PostgreSQL operations
│   └── qdrant_repo.go   # Vector search operations (gRPC)
├── storage/minio.go     # S3-compatible object storage
├── source/              # Data source adapters (extensible)
└── domain/              # Data models (Meme, Source, Job)
```

### Data Flow

1. **Ingestion**: Source adapter → VLM description → Jina embedding → MinIO upload → Qdrant upsert → SQLite save
2. **Search**: Query text → Jina embedding → Qdrant cosine similarity → Return top-K results

## API Endpoints

- `POST /api/v1/search` - Semantic meme search (`{"query": "text", "top_k": 20}`)
- `GET /api/v1/categories` - List categories
- `GET /api/v1/memes` - List memes (supports `category`, `limit`, `offset`)
- `GET /api/v1/memes/{id}` - Get meme details
- `GET /api/v1/stats` - System statistics
- `GET /health` - Health check

## Configuration

Environment variables (see `.env.example`):
- `OPENAI_API_KEY`, `OPENAI_BASE_URL` - VLM provider
- `JINA_API_KEY` - Embeddings API
- `MINIO_*` - Object storage credentials
- `QDRANT_HOST`, `QDRANT_PORT` - Vector DB connection
- `DATABASE_PATH` - SQLite path

Config file: `configs/config.yaml`

## Key Patterns

- **Source Interface**: New data sources implement `Source` interface in `internal/source/`
- **Worker Pool**: Ingest service uses goroutine workers with configurable concurrency
- **Layered Architecture**: Handler → Service → Repository → Storage
- **Meme Status**: `pending` (awaiting VLM) → `active` (ready) or `failed`
