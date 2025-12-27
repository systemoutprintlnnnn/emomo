#!/bin/bash

set -e

echo "=== Emomo Setup Script ==="

# Check if .env exists
if [ ! -f .env ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo "Please edit .env to add your API keys before running the services."
fi

# Create data directory
mkdir -p data

# Check if ChineseBQB is cloned
if [ ! -d "data/ChineseBQB" ]; then
    echo "Cloning ChineseBQB repository..."
    git clone https://github.com/zhaoolee/ChineseBQB.git ./data/ChineseBQB
fi

# Build binaries
echo "Building API server..."
go build -o api ./cmd/api

echo "Building ingest tool..."
go build -o ingest ./cmd/ingest

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "1. Edit .env to add your API keys (OPENAI_API_KEY, JINA_API_KEY)"
echo "2. Start infrastructure: docker-compose -f deployments/docker-compose.yml up -d"
echo "3. Run ingestion: ./ingest --source=chinesebqb --limit=100"
echo "4. Start API server: ./api"
