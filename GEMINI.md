# GEMINI.md - Repository-wide Context for AI Assistants

This is the emomo monorepo: an AI-powered meme search engine with backend and frontend subprojects. Subproject-specific GEMINI.md files contain the details:

- [backend/GEMINI.md](backend/GEMINI.md) — Go backend (REST API + ingestion + Qdrant/storage)
- [frontend/GEMINI.md](frontend/GEMINI.md) — React + Vite frontend

## 1. Repo Layout

```
backend/      Go 1.24 + Gin, ingestion + REST API
frontend/    React 19 + Vite SPA
deployments/ Cross-service Docker Compose (API + Grafana Alloy)
docs/        Cross-service design and ops docs
scripts/     Cross-service helpers (start.sh)
```

## 2. End-to-End Data Flow

```mermaid
graph LR
    Local[Local Static Image Dir] --> Ingest[Ingest Service]

    Ingest -->|upload| S3[Object Storage]
    Ingest -->|VLM and Embed| AI[AI Services]
    Ingest -->|metadata| DB[(PostgreSQL)]
    Ingest -->|vectors| Vector[(Qdrant)]

    User -->|search| API[Backend API]
    Frontend -->|HTTP| API
    API -->|query| AI
    API -->|search| Vector
    API -->|fetch| DB
```

## 3. Common Tasks

### Local dev

```bash
./scripts/start.sh                 # backend (8080) + frontend (5173)
docker compose -f deployments/docker-compose.yml up -d   # API + Alloy
```

### Per-subproject

```bash
cd backend && go run ./cmd/api
cd frontend && npm run dev
cd backend && ./scripts/import-data.sh -p ./data/memes -l 50
```

## 4. Conventions

- Branch from `main`. Use Conventional Commits prefixes (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`).
- Cross-subproject changes: scope by directory in the commit body.
- Each subproject has its own `.env.example`. Don't put secrets in repo root.
- The Go module path (`github.com/timmy/emomo`) is independent of file paths; moving files does not require import rewrites.

## 5. Deployment Surfaces

- Render — `render.yaml` (`rootDir: backend`).
- Railway — `railway.json` (`dockerfilePath: backend/Dockerfile`).
- Hugging Face Space — GitHub Actions splits `backend/` as a subtree and force-pushes to the Space; the Space's filesystem root equals `backend/`.

## 6. Testing

- Backend: `cd backend && go test ./...`.
- Frontend: `cd frontend && npm run test` (Playwright e2e).
