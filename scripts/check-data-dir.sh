#!/bin/sh
# Check if ChineseBQB directory exists and has files
DATA_DIR="${DATA_DIR:-/root/data}"
CHINESEBQB_DIR="${DATA_DIR}/ChineseBQB"

echo "=========================================="
echo "Checking data directories..."
echo "=========================================="
echo "Data directory: ${DATA_DIR}"
echo "ChineseBQB directory: ${CHINESEBQB_DIR}"
echo ""

if [ ! -d "${DATA_DIR}" ]; then
    echo "ERROR: Data directory does not exist: ${DATA_DIR}"
    echo "Please ensure the data directory is mounted in docker-compose.yml"
    exit 1
fi

if [ ! -d "${CHINESEBQB_DIR}" ]; then
    echo "ERROR: ChineseBQB directory does not exist: ${CHINESEBQB_DIR}"
    echo ""
    echo "Please ensure:"
    echo "1. The ChineseBQB directory exists on the host at: <project-root>/data/ChineseBQB"
    echo "2. The volume mount in docker-compose.prod.yml is correct: ../data:/root/data"
    echo "3. The directory contains image files"
    exit 1
fi

FILE_COUNT=$(find "${CHINESEBQB_DIR}" -type f \( -name "*.jpg" -o -name "*.jpeg" -o -name "*.png" -o -name "*.gif" -o -name "*.webp" \) 2>/dev/null | wc -l)

if [ "${FILE_COUNT}" -eq 0 ]; then
    echo "WARNING: ChineseBQB directory exists but contains no image files"
    echo "Directory: ${CHINESEBQB_DIR}"
else
    echo "✓ ChineseBQB directory found with ${FILE_COUNT} image files"
fi

echo "=========================================="

