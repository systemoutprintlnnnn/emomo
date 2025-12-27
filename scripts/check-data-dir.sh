#!/bin/sh
# Check if ChineseBQB directory exists and has files
# If empty or missing, automatically clone from GitHub
# Note: ChineseBQB contains subdirectories with Chinese/emoji characters

DATA_DIR="${DATA_DIR:-/root/data}"
CHINESEBQB_DIR="${DATA_DIR}/ChineseBQB"
CHINESEBQB_REPO="https://github.com/zhaoolee/ChineseBQB.git"

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

# Function to check if directory has image files
check_images() {
    local dir="$1"
    local count=$(find "${dir}" -type f \( -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.png" -o -iname "*.gif" -o -iname "*.webp" \) 2>/dev/null | wc -l)
    echo "$count"
}

# Check if ChineseBQB directory exists and has content
NEED_CLONE=false

if [ ! -d "${CHINESEBQB_DIR}" ]; then
    echo "ChineseBQB directory does not exist"
    NEED_CLONE=true
elif [ -z "$(ls -A "${CHINESEBQB_DIR}" 2>/dev/null)" ]; then
    echo "ChineseBQB directory exists but is empty"
    NEED_CLONE=true
else
    FILE_COUNT=$(check_images "${CHINESEBQB_DIR}")
    if [ "${FILE_COUNT}" -eq 0 ]; then
        echo "ChineseBQB directory exists but contains no image files"
        NEED_CLONE=true
    fi
fi

# Clone ChineseBQB repository if needed
if [ "${NEED_CLONE}" = "true" ]; then
    echo ""
    echo "=========================================="
    echo "Cloning ChineseBQB repository from GitHub..."
    echo "Repository: ${CHINESEBQB_REPO}"
    echo "This may take a few minutes..."
    echo "=========================================="
    echo ""
    
    # Remove empty directory if exists
    if [ -d "${CHINESEBQB_DIR}" ]; then
        rm -rf "${CHINESEBQB_DIR}"
    fi
    
    # Clone the repository
    if git clone --depth 1 "${CHINESEBQB_REPO}" "${CHINESEBQB_DIR}"; then
        echo ""
        echo "✓ Successfully cloned ChineseBQB repository"
    else
        echo ""
        echo "ERROR: Failed to clone ChineseBQB repository"
        echo "Please check your network connection and try again"
        echo ""
        echo "You can also manually clone on the host machine:"
        echo "  git clone ${CHINESEBQB_REPO} ./data/ChineseBQB"
        echo ""
        echo "Continuing without ChineseBQB data..."
        echo "The API will start but ingestion will fail."
        exit 0
    fi
fi

# Count subdirectories and files
SUBDIR_COUNT=$(find "${CHINESEBQB_DIR}" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
FILE_COUNT=$(check_images "${CHINESEBQB_DIR}")

echo ""
echo "=========================================="
echo "ChineseBQB Data Summary"
echo "=========================================="
echo "Subdirectories: ${SUBDIR_COUNT}"
echo "Image files: ${FILE_COUNT}"

if [ "${FILE_COUNT}" -eq 0 ]; then
    echo ""
    echo "WARNING: No image files found!"
    echo "This may be caused by:"
    echo "1. Unicode/Chinese directory names not being read correctly"
    echo "2. Clone was incomplete"
    echo ""
    echo "Listing first level contents:"
    ls -la "${CHINESEBQB_DIR}" 2>/dev/null | head -20
else
    echo "✓ ChineseBQB data is ready"
fi

echo "=========================================="
echo ""

