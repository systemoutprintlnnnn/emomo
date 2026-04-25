# Data Ingest

Emomo currently ingests memes from a local ChineseBQB checkout. The older web-scrape staging pipeline has been removed, so `cmd/ingest` accepts `chinesebqb` as the only source.

## Prepare Data

From `backend/`:

```bash
git clone https://github.com/zhaoolee/ChineseBQB.git ./data/ChineseBQB
```

The default source path is configured in `backend/configs/config.yaml`:

```yaml
sources:
  chinesebqb:
    enabled: true
    repo_path: ./data/ChineseBQB
```

## Run Ingest

```bash
cd backend
./scripts/import-data.sh -s chinesebqb -l 100
```

Equivalent direct command:

```bash
go run ./cmd/ingest --source=chinesebqb --limit=100
```

Use `--embedding` to select a non-default embedding configuration:

```bash
./scripts/import-data.sh -s chinesebqb -e jina -l 100
./scripts/import-data.sh -s chinesebqb -e qwen3 -l 100
```

Use `--force` to reprocess existing source records, or `--retry` to process pending records:

```bash
./scripts/import-data.sh -s chinesebqb -f
./scripts/import-data.sh -r -l 100
```
