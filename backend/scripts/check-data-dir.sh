#!/bin/sh
# Check if the local static image directory exists.

DATA_DIR="${DATA_DIR:-/root/data}"
LOCAL_MEMES_DIR="${LOCAL_MEMES_DIR:-${DATA_DIR}/memes}"

echo "===== Application Startup at $(date '+%Y-%m-%d %H:%M:%S') ====="
echo ""
echo "=========================================="
echo "Checking data directories..."
echo "=========================================="
echo "Data directory: ${DATA_DIR}"
echo "Local static image directory: ${LOCAL_MEMES_DIR}"
echo "Locale: ${LANG:-not set}"
echo ""

# Ensure data directory exists
if [ ! -d "${DATA_DIR}" ]; then
    echo "Creating data directory: ${DATA_DIR}"
    mkdir -p "${DATA_DIR}"
fi

# Check if local static image directory exists
if [ ! -d "${LOCAL_MEMES_DIR}" ]; then
    echo "Local static image directory does not exist at ${LOCAL_MEMES_DIR}"
    echo "Create it or set LOCAL_MEMES_DIR / sources.localdir.root_path before ingesting."
else
    echo "Local static image directory found."
    # Optional: Count files if needed, or just skip it
    FILE_COUNT=$(find "${LOCAL_MEMES_DIR}" -type f 2>/dev/null | wc -l)
    echo "Found ${FILE_COUNT} files in ${LOCAL_MEMES_DIR}"
fi

echo "=========================================="
echo "Startup check complete"
echo "=========================================="
echo ""
