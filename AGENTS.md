# Repository Guidelines

## Project Structure & Module Organization
- `cmd/`: Go entry points (`cmd/api`, `cmd/ingest`).
- `internal/`: Go application code (API handlers, services, repositories, sources, storage).
- `crawler/`: Python crawler managed by `uv` (CLI under `crawler/src/emomo_crawler/`).
- `configs/`: YAML config files and examples.
- `deployments/`: Docker Compose and deployment configs.
- `data/`: Local data and staging directories (do not commit large artifacts).
- `docs/`: Design and usage documentation.

## Build, Test, and Development Commands
- `docker-compose -f deployments/docker-compose.yml up -d`: start local infra (Qdrant + storage).
- `go run ./cmd/api`: run the API server locally.
- `go build -o ingest ./cmd/ingest`: build the ingestion CLI.
- `./ingest --source=staging:fabiaoqing --limit=50`: ingest staged memes.
- `cd crawler && uv sync`: install crawler dependencies.
- `cd crawler && uv run emomo-crawler crawl --source fabiaoqing --limit 100`: crawl into `data/staging/`.

## Coding Style & Naming Conventions
- Go: follow `gofmt` defaults (tabs for indentation); package names are short and lowercase.
- Python: standard 4-space indentation; keep crawler modules in `snake_case`.
- Config: keep new keys grouped by subsystem under `configs/`.

## Testing Guidelines
- Go tests: `go test ./...`.
- No dedicated Python test suite is documented for the crawler yet; add tests alongside new crawler features if you introduce them.

## Commit & Pull Request Guidelines
- Commit messages follow a Conventional Commits-style prefix (e.g., `chore:`, `docs:`, `feat:`). Keep the subject short and imperative.
- PRs should include a clear description of scope, link related issues, and add screenshots or curl examples for API changes.

## Security & Configuration Tips
- Never commit API keys or secrets; use `.env` or environment variables.
- For production, prefer the `deployments/docker-compose.prod.yml` workflows and TLS-enabled endpoints.
