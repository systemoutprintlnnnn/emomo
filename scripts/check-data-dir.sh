#!/bin/sh
# Check if ChineseBQB directory exists and has files
# Note: ChineseBQB contains subdirectories with Chinese/emoji characters

DATA_DIR="${DATA_DIR:-/root/data}"
CHINESEBQB_DIR="${DATA_DIR}/ChineseBQB"

echo "=========================================="
echo "Checking data directories..."
echo "=========================================="
echo "Data directory: ${DATA_DIR}"
echo "ChineseBQB directory: ${CHINESEBQB_DIR}"
echo "Locale: ${LANG:-not set}"
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

# Count subdirectories (ChineseBQB has images in subdirectories like "001Funny_滑稽大佬😏BQB/")
SUBDIR_COUNT=$(find "${CHINESEBQB_DIR}" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
echo "Subdirectories found: ${SUBDIR_COUNT}"

# Count image files recursively in all subdirectories
FILE_COUNT=$(find "${CHINESEBQB_DIR}" -type f \( -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.png" -o -iname "*.gif" -o -iname "*.webp" \) 2>/dev/null | wc -l)

if [ "${FILE_COUNT}" -eq 0 ]; then
    echo ""
    echo "WARNING: ChineseBQB directory exists but contains no image files"
    echo "Directory: ${CHINESEBQB_DIR}"
    echo ""
    echo "This may be caused by:"
    echo "1. Unicode/Chinese directory names not being read correctly"
    echo "2. Missing locale support (LANG=${LANG:-not set})"
    echo "3. The directory was cloned without Git LFS"
    echo ""
    echo "Listing first level contents:"
    ls -la "${CHINESEBQB_DIR}" 2>/dev/null | head -20
else
    echo "✓ ChineseBQB directory found with ${FILE_COUNT} image files"
fi

echo "=========================================="

