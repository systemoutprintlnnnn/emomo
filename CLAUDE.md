# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Emomo is an AI-powered meme/sticker semantic search system. Users can search for memes using natural language queries in Chinese.

**Tech Stack:** Go 1.24 + Gin, Qdrant (vector DB), S3-compatible storage (R2/S3), PostgreSQL, OpenAI-compatible VLM (e.g., GPT-4o mini), Jina Embeddings v3

## Build & Run Commands

```bash
# Build binaries
go build -o api ./cmd/api
go build -o ingest ./cmd/ingest

# Start infrastructure (Qdrant, object storage can use cloud services like Cloudflare R2)
docker-compose -f deployments/docker-compose.yml up -d

# Data ingestion (static sources)
./ingest --source=chinesebqb --limit=100    # Ingest memes
./ingest --retry --limit=100                # Retry pending items
./ingest --force --source=chinesebqb        # Force re-process

# Data ingestion (from staging, after crawler)
./ingest --source=staging:fabiaoqing --limit=50

# Run API server (port 8080)
./api

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
./ingest --source=staging:fabiaoqing --limit=50
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

1. **Ingestion**: Source adapter → VLM description → Jina embedding → Object storage upload → Qdrant upsert → PostgreSQL save
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
- `OPENAI_API_KEY`, `OPENAI_BASE_URL` - VLM provider (OpenAI compatible)
- `JINA_API_KEY` - Embeddings API
- `STORAGE_*` - Object storage credentials (supports R2, S3, etc.)
- `QDRANT_HOST`, `QDRANT_PORT` - Vector DB connection
- `DATABASE_URL` - PostgreSQL connection URL

Config file: `configs/config.yaml`

## Key Patterns

- **Source Interface**: New data sources implement `Source` interface in `internal/source/`
- **Worker Pool**: Ingest service uses goroutine workers with configurable concurrency
- **Layered Architecture**: Handler → Service → Repository → Storage
- **Meme Status**: `pending` (awaiting VLM) → `active` (ready) or `failed`
