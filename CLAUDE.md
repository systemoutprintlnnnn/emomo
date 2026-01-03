# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Emomo is an AI-powered meme/sticker semantic search system. Users can search for memes using natural language queries in Chinese.

**Tech Stack:** Go 1.24 + Gin, Qdrant (vector DB), S3-compatible storage (R2/S3), SQLite/PostgreSQL, OpenAI-compatible VLM (e.g., GPT-4o mini), Jina Embeddings v3, Grafana Alloy + Loki (logging)

## Build & Run Commands

```bash
# Build binaries (optional, can use go run instead)
go build -o api ./cmd/api

# Start infrastructure (API + Alloy; Qdrant/S3 are external)
docker-compose -f deployments/docker-compose.yml up -d

# Logs only (Grafana Alloy)
docker-compose -f deployments/docker-compose.yml up -d alloy

# Data ingestion using script (recommended, no build required)
./scripts/import-data.sh -s chinesebqb -l 100       # Ingest memes
./scripts/import-data.sh -r -l 100                  # Retry pending items
./scripts/import-data.sh -s chinesebqb -f           # Force re-process
./scripts/import-data.sh -s staging:fabiaoqing -l 50  # From staging

# Or use go run directly
go run ./cmd/ingest --source=chinesebqb --limit=100

# Run API server (port 8080)
go run ./cmd/api

# Full stack (backend + frontend)
./scripts/start.sh
```

## Python Crawler

The `crawler/` directory contains a Python-based meme crawler using requests + BeautifulSoup.

```bash
# Setup crawler
cd crawler
uv sync

# Crawl memes to staging
uv run emomo-crawler crawl --source fabiaoqing --limit 100

# Continue from a specific page
uv run emomo-crawler crawl --source fabiaoqing --limit 100 --cursor 10

# View staging status
uv run emomo-crawler staging list
uv run emomo-crawler staging stats --source fabiaoqing

# Import from staging to main system
cd ..
./scripts/import-data.sh -s staging:fabiaoqing -l 50
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
│   ├── vlm.go           # OpenAI-compatible VLM client for image descriptions
│   └── embedding.go     # Jina text embeddings (1024-dim)
├── repository/
│   ├── meme_repo.go     # PostgreSQL operations
│   └── qdrant_repo.go   # Vector search operations (gRPC)
├── storage/s3.go        # S3-compatible object storage (supports R2, S3, etc.)
├── source/              # Data source adapters (extensible)
│   ├── chinesebqb/      # Static file system source
│   └── staging/         # Staging directory source (from Python crawler)
└── domain/              # Data models (Meme, Source, Job)

crawler/                 # Python crawler (requests + BeautifulSoup)
├── src/emomo_crawler/
│   ├── cli.py           # CLI commands
│   ├── staging.py       # Staging area management
│   ├── base.py          # Base crawler class
│   └── sources/         # Crawler implementations
└── pyproject.toml
```

### Data Flow

1. **Ingestion**: Source adapter → VLM description → Jina embedding → Object storage upload → Qdrant upsert → Database save
2. **Search**: Query text → (Optional: VLM query expansion) → Jina embedding → Qdrant cosine similarity → Return top-K results

## API Endpoints

- `POST /api/v1/search` - Semantic meme search (`{"query": "text", "top_k": 20}`)
- `GET /api/v1/categories` - List categories
- `GET /api/v1/memes` - List memes (supports `category`, `limit`, `offset`)
- `GET /api/v1/memes/{id}` - Get meme details
- `GET /api/v1/stats` - System statistics
- `GET /health` - Health check

## Configuration

Environment variables (see `.env.example`):
- **VLM**: `VLM_MODEL`, `OPENAI_API_KEY`, `OPENAI_BASE_URL` - Vision-language model for image descriptions
- **Embeddings**: `EMBEDDING_MODEL`, `JINA_API_KEY` - Text embedding service
- **Search**: `QUERY_EXPANSION_MODEL` - Optional VLM for query enhancement
- **Storage**: `STORAGE_ENDPOINT`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_PUBLIC_URL` - S3/R2 configuration
- **Qdrant**: `QDRANT_HOST`, `QDRANT_PORT`, `QDRANT_API_KEY`, `QDRANT_USE_TLS` - Vector database
- **Database**: `DATABASE_DRIVER` (sqlite/postgres), `DATABASE_PATH` or `DATABASE_URL` - Relational DB
- **Monitoring**: `LOKI_URL`, `LOKI_USERNAME`, `LOKI_PASSWORD`, `CLUSTER_NAME`, `ENVIRONMENT` - Grafana Cloud logging

Config file: `configs/config.yaml`

## Deployment & Monitoring

- **Compose**: Run `docker-compose.yml` for API + Alloy (Qdrant/S3 are external)
- **Logging only**: Run `docker-compose.yml` with `alloy` service if you only need log collection
- **Logging**: Grafana Alloy collects Docker container logs and forwards to Grafana Cloud Loki
- **Observability**: Alloy UI available at `http://localhost:12345` for pipeline monitoring

## Key Patterns

- **Source Interface**: New data sources implement `Source` interface in `internal/source/`
- **Worker Pool**: Ingest service uses goroutine workers with configurable concurrency
- **Layered Architecture**: Handler → Service → Repository → Storage
- **Meme Status**: `pending` (awaiting VLM) → `active` (ready) or `failed`
