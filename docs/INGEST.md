# Data Ingest

Emomo ingests meme resources from a local static image directory. GIF is not supported; only static `.jpg`, `.jpeg`, `.png`, and `.webp` files are scanned.

## Prepare Data

From `backend/`, place images under `data/memes`:

```text
backend/data/memes/
├── 猫猫/
│   ├── 无语.jpg
│   └── 开心.png
└── 狗狗/
    └── 柴犬.webp
```

The default source path is configured in `backend/configs/config.yaml`:

```yaml
sources:
  localdir:
    enabled: true
    root_path: ./data/memes
    source_id: localdir
```

You can also override the path with `LOCAL_MEMES_DIR` or the CLI `--path` flag.

## Run Ingest

```bash
cd backend
./scripts/import-data.sh -p ./data/memes -l 100
```

Equivalent direct command:

```bash
go run ./cmd/ingest --source=localdir --path=./data/memes --limit=100
```

Use `--embedding` to select a non-default embedding configuration:

```bash
./scripts/import-data.sh -p ./data/memes -e jina -l 100
./scripts/import-data.sh -p ./data/memes -e qwen3 -l 100
```

Use `--force` to reprocess existing source records, or `--retry` to process pending records:

```bash
./scripts/import-data.sh -p ./data/memes -f
./scripts/import-data.sh -r -l 100
```

## Metadata Rules

- `source_id`: relative file path, for example `猫猫/无语.jpg`.
- `category`: first-level directory name; files directly under the root use `未分类`.
- `format`: detected from extension first, then verified by magic bytes during ingestion.
- unsupported formats, including GIF, are skipped or rejected before persistence.
