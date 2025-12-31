#!/bin/bash

set -euo pipefail

SOURCE="${1:-fabiaoqing}"
LIMIT="${2:-100}"

echo "=== Emomo Staging Ingest ==="
echo "Source: staging:${SOURCE}"
echo "Limit: ${LIMIT}"

if [ ! -f ingest ]; then
    echo "Building ingest tool..."
    go build -o ingest ./cmd/ingest
fi

./ingest --source="staging:${SOURCE}" --limit="${LIMIT}"
