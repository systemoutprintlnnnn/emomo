# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Emomo is an AI-powered meme/sticker semantic search system. Users can search for memes using natural language queries in Chinese.

**Tech Stack:** Go 1.24 + Gin, Qdrant (vector DB), S3-compatible storage (R2/S3), SQLite/PostgreSQL, OpenAI-compatible VLM (e.g., GPT-4o mini), Jina Embeddings v3, Grafana Alloy + Loki (logging)

## Build & Run Commands

```bash
# Build binaries (optional, can use go run instead)
go build -o api ./cmd/api
go build -o ingest ./cmd/ingest

# Run tests
go test ./...

# Run specific test
go test ./internal/service -run TestQueryUnderstandingService

# Start infrastructure (API + Alloy; Qdrant/S3 are external)
docker-compose -f deployments/docker-compose.yml up -d

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
│   ├── search.go        # Semantic search with multi-collection support
│   ├── query_understanding.go  # Query intent analysis & hybrid search routing
│   ├── ingest.go        # Ingestion pipeline with worker pool
│   ├── vlm.go           # OpenAI-compatible VLM client for image descriptions
│   └── embedding.go     # Text embeddings (supports multiple providers)
├── repository/
│   ├── meme_repo.go     # PostgreSQL/SQLite operations
│   ├── meme_description_repo.go  # VLM-generated descriptions
│   └── qdrant_repo.go   # Vector search operations (gRPC)
├── storage/s3.go        # S3-compatible object storage (supports R2, S3, etc.)
├── source/              # Data source adapters (extensible)
│   ├── interface.go     # Source interface definition
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

1. **Ingestion**: Source adapter → VLM description → Text embedding → Object storage upload → Qdrant upsert → Database save
2. **Search**: Query text → Query understanding (optional) → Hybrid search (BM25 + vector) → RRF fusion → Return top-K results

### Key Patterns

#### Multi-Collection Support
The system supports multiple embedding models, each with its own Qdrant collection. Collections are configured in `configs/config.yaml` under `embeddings[]`. Each collection can use a different provider (jina, openai, modelscope), model, dimensions, and collection name. The default collection is marked with `is_default: true`.

#### Query Understanding & Hybrid Search
Query understanding analyzes user intent and routes to appropriate search strategy:
- **Intent Classification**: Emotion, Subject, Scene, Meme, Text, Composite, Semantic
- **Hybrid Search**: Combines BM25 (sparse) + vector (dense) search with Reciprocal Rank Fusion (RRF)
- **Adaptive Weights**: Dense/sparse weights adjust based on intent (emotion → high dense weight, text → low dense weight)
- **Fallback Logic**: If VLM-based understanding is disabled or fails, uses rule-based classification

Query understanding uses streaming JSON parsing to handle partial LLM responses.

#### Source Interface Pattern
New data sources must implement the `Source` interface in `internal/source/interface.go`:
- `GetSourceID()` - Unique identifier
- `GetDisplayName()` - Human-readable name
- `FetchBatch(ctx, cursor, limit)` - Paginated item fetching
- `SupportsIncremental()` - Whether incremental updates are supported

#### Deterministic Vector IDs
Qdrant point IDs are deterministically generated using MD5 hash of (source_type, source_id, embedding_name) to ensure idempotency and enable safe re-ingestion without duplicates.

#### Meme Status Lifecycle
- `pending` - Awaiting VLM description generation
- `active` - Description generated, vector embedded, ready for search
- `failed` - VLM or embedding generation failed

#### Worker Pool Ingestion
Ingest service uses configurable goroutine workers (default: 5) with batch processing (default: 10) and retry logic (default: 3 retries).

## API Endpoints

- `POST /api/v1/search` - Semantic meme search (`{"query": "text", "top_k": 20, "collection": "emomo"}`)
- `GET /api/v1/categories` - List categories
- `GET /api/v1/memes` - List memes (supports `category`, `limit`, `offset`)
- `GET /api/v1/memes/{id}` - Get meme details
- `GET /api/v1/stats` - System statistics
- `GET /health` - Health check

## Configuration

Config file: `configs/config.yaml`
Environment variables: See `.env.example`

Key environment variables:
- **VLM**: `VLM_MODEL`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`
- **Embeddings**: `EMBEDDING_API_KEY`, `JINA_API_KEY`, `MODELSCOPE_API_KEY`
- **Query Understanding**: `QUERY_EXPANSION_MODEL`, `QUERY_EXPANSION_API_KEY`, `QUERY_EXPANSION_BASE_URL`
- **Storage**: `STORAGE_ENDPOINT`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_PUBLIC_URL`
- **Qdrant**: `QDRANT_HOST`, `QDRANT_PORT`, `QDRANT_API_KEY`, `QDRANT_USE_TLS`
- **Database**: `DATABASE_DRIVER` (sqlite/postgres), `DATABASE_PATH` or `DATABASE_URL`
- **Monitoring**: `LOKI_URL`, `LOKI_USERNAME`, `LOKI_PASSWORD`, `CLUSTER_NAME`, `ENVIRONMENT`

## Deployment & Monitoring

- **Compose**: Run `docker-compose.yml` for API + Alloy (Qdrant/S3 are external)
- **Logging**: Grafana Alloy collects Docker container logs and forwards to Grafana Cloud Loki
- **Observability**: Alloy UI available at `http://localhost:12345` for pipeline monitoring

## Extended Documentation

For detailed design documentation:
- `docs/QUERY_UNDERSTANDING_DESIGN.md` - Query understanding architecture and hybrid search
- `docs/MULTI_EMBEDDING.md` - Multi-collection embedding configuration
- `docs/DATA_INGEST_ARCHITECTURE.md` - Ingestion pipeline details
- `docs/DATABASE_SCHEMA.md` - Database schema and migrations
- `docs/DEPLOYMENT.md` - Production deployment guide
- `docs/QUICK_START.md` - Quick start guide for local development
