#!/bin/sh
# Check if ChineseBQB directory exists
# Note: Automatic cloning has been disabled per user request

DATA_DIR="${DATA_DIR:-/root/data}"
CHINESEBQB_DIR="${DATA_DIR}/ChineseBQB"

echo "===== Application Startup at $(date '+%Y-%m-%d %H:%M:%S') ====="
echo ""
echo "=========================================="
echo "Checking data directories..."
echo "=========================================="
echo "Data directory: ${DATA_DIR}"
echo "ChineseBQB directory: ${CHINESEBQB_DIR}"
echo "Locale: ${LANG:-not set}"
echo ""

# Ensure data directory exists
if [ ! -d "${DATA_DIR}" ]; then
    echo "Creating data directory: ${DATA_DIR}"
    mkdir -p "${DATA_DIR}"
fi

# Check if ChineseBQB directory exists
if [ ! -d "${CHINESEBQB_DIR}" ]; then
    echo "ChineseBQB directory does not exist at ${CHINESEBQB_DIR}"
    echo "Automatic cloning is disabled."
else
    echo "ChineseBQB directory found."
    # Optional: Count files if needed, or just skip it
    FILE_COUNT=$(find "${CHINESEBQB_DIR}" -type f 2>/dev/null | wc -l)
    echo "Found ${FILE_COUNT} files in ${CHINESEBQB_DIR}"
fi

echo "=========================================="
echo "Startup check complete"
echo "=========================================="
echo ""