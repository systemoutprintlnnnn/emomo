# Emomo Crawler

Python-based meme crawler for the emomo project, using [Crawl4AI](https://crawl4ai.com).

## Installation

```bash
# Install uv if not already installed
curl -LsSf https://astral.sh/uv/install.sh | sh

# Navigate to crawler directory
cd crawler

# Install dependencies
uv sync

# Setup Crawl4AI (downloads browser)
uv run crawl4ai-setup
```

## Usage

### Crawl Memes

```bash
# Crawl 100 memes from fabiaoqing.com
uv run emomo-crawler crawl --source fabiaoqing --limit 100

# Continue from a specific page
uv run emomo-crawler crawl --source fabiaoqing --limit 100 --cursor 5

# Adjust rate limit (requests per second)
uv run emomo-crawler crawl --source fabiaoqing --limit 50 --rate-limit 1.0
```

### Manage Staging

```bash
# List all sources in staging
uv run emomo-crawler staging list

# View statistics for a source
uv run emomo-crawler staging stats --source fabiaoqing

# Clean staging for a source
uv run emomo-crawler staging clean --source fabiaoqing

# Clean all staging data
uv run emomo-crawler staging clean-all
```

## Import to Emomo

After crawling, import from staging using the Go ingest tool:

```bash
# Build the ingest tool
go build -o ingest ./cmd/ingest

# Import from staging
./ingest --source=staging:fabiaoqing --limit=50
```

## Project Structure

```
crawler/
├── pyproject.toml              # Project config (uv/pip)
├── README.md
└── src/
    └── emomo_crawler/
        ├── __init__.py
        ├── cli.py              # CLI commands
        ├── config.py           # Configuration
        ├── staging.py          # Staging area management
        ├── base.py             # Base crawler class
        └── sources/
            ├── __init__.py
            └── fabiaoqing.py   # Fabiaoqing crawler
```

## Staging Format

Crawled memes are stored in `data/staging/{source}/`:

```
data/staging/fabiaoqing/
├── manifest.jsonl      # Metadata (one JSON per line)
└── images/
    ├── abc123.jpg
    └── def456.png
```

Each line in `manifest.jsonl`:

```json
{"id": "abc123", "filename": "abc123.jpg", "category": "熊猫头", "tags": ["搞笑"], "source_url": "https://...", "is_animated": false, "format": "jpg", "crawled_at": "2024-01-01T00:00:00Z"}
```

## Adding New Sources

1. Create a new file in `sources/`, e.g., `sources/doutula.py`
2. Implement a class extending `BaseCrawler`
3. Register in `sources/__init__.py`
4. Add to `CRAWLERS` dict in `cli.py`

Example:

```python
from ..base import BaseCrawler

class DoutulaCrawler(BaseCrawler):
    @property
    def source_id(self) -> str:
        return "doutula"

    @property
    def display_name(self) -> str:
        return "斗图啦 (doutula.com)"

    async def crawl(self, staging, limit, cursor=None):
        # Implementation
        pass
```
