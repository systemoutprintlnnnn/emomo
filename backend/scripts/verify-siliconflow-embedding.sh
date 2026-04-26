#!/usr/bin/env bash

# Verify SiliconFlow embeddings model availability and input payload shape.
#
# Required:
#   SILICONFLOW_API_KEY=... (exported or defined in backend/.env)
#
# Optional:
#   ENV_FILE=/path/to/.env
#   SILICONFLOW_BASE_URL=https://api.siliconflow.cn/v1
#   SILICONFLOW_EMBEDDING_MODEL=Qwen/Qwen3-VL-Embedding-8B
#   SILICONFLOW_EMBEDDING_DIMENSIONS=1024
#   TEST_IMAGE_URL=https://upload.wikimedia.org/wikipedia/commons/3/3f/JPEG_example_flower.jpg

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${ENV_FILE:-${SCRIPT_DIR}/../.env}"

load_env_file() {
    local env_file="$1"
    local line key value

    [ -f "$env_file" ] || return 0

    while IFS= read -r line || [ -n "$line" ]; do
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"

        case "$line" in
            ""|\#*) continue ;;
        esac

        if [[ "$line" == export\ * ]]; then
            line="${line#export }"
        fi
        if [[ "$line" != *=* ]]; then
            continue
        fi

        key="${line%%=*}"
        value="${line#*=}"
        key="${key%"${key##*[![:space:]]}"}"
        key="${key#"${key%%[![:space:]]*}"}"

        if [[ ! "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
            continue
        fi
        if [ "${!key+x}" ]; then
            continue
        fi

        if [[ "$value" =~ ^\"(.*)\"$ ]]; then
            value="${BASH_REMATCH[1]}"
        elif [[ "$value" =~ ^\'(.*)\'$ ]]; then
            value="${BASH_REMATCH[1]}"
        fi

        export "$key=$value"
    done < "$env_file"
}

load_env_file "$ENV_FILE"

BASE_URL="${SILICONFLOW_BASE_URL:-https://api.siliconflow.cn/v1}"
EMBEDDING_MODEL="${SILICONFLOW_EMBEDDING_MODEL:-Qwen/Qwen3-VL-Embedding-8B}"
EMBEDDING_DIMENSIONS="${SILICONFLOW_EMBEDDING_DIMENSIONS:-1024}"
TEST_IMAGE_URL="${TEST_IMAGE_URL:-https://upload.wikimedia.org/wikipedia/commons/3/3f/JPEG_example_flower.jpg}"
QUERY_TEXT="测试表情包搜索"

usage() {
    cat <<EOF
Usage:
  $0
  SILICONFLOW_API_KEY=... $0

Environment:
  ENV_FILE            Env file to load first, default: ${ENV_FILE}
  SILICONFLOW_BASE_URL  API base URL, default: ${BASE_URL}
  SILICONFLOW_EMBEDDING_MODEL       Model to validate, default: ${EMBEDDING_MODEL}
  SILICONFLOW_EMBEDDING_DIMENSIONS  Output dimensions, default: ${EMBEDDING_DIMENSIONS}
  TEST_IMAGE_URL        Public image URL downloaded and converted to a data URI for image checks

Checks:
  1. Model list contains ${EMBEDDING_MODEL}.
  2. Text string input works for ${EMBEDDING_MODEL}.
  3. Current text object input {"text": "..."} works for ${EMBEDDING_MODEL}.
  4. Current batch text object input [{"text": "..."}, ...] works for ${EMBEDDING_MODEL}.
  5. Current image object input {"image": "data:image/...;base64,..."} works and reports image_tokens > 0.
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
fi

if [ -z "${SILICONFLOW_API_KEY:-}" ]; then
    echo "SILICONFLOW_API_KEY is required." >&2
    usage >&2
    exit 2
fi

dimensions_field() {
    local dimensions="$1"
    if [ -n "$dimensions" ]; then
        printf ',"dimensions":%s' "$dimensions"
    fi
}

embedding_payload() {
    local model="$1"
    local input_json="$2"
    local dimensions="$3"
    printf '{"model":"%s","input":%s,"encoding_format":"float"%s}' \
        "$model" "$input_json" "$(dimensions_field "$dimensions")"
}

image_data_uri() {
    local image_url="$1"
    local image_file
    local media_type
    local encoded

    image_file="$(mktemp)"
    if ! curl -fsSL "$image_url" -o "$image_file"; then
        rm -f "$image_file"
        return 1
    fi

    media_type="$(file -b --mime-type "$image_file" 2>/dev/null || true)"
    if [[ "$media_type" != image/* ]]; then
        media_type="image/jpeg"
    fi
    encoded="$(base64 < "$image_file" | tr -d '\n')"
    rm -f "$image_file"
    printf 'data:%s;base64,%s' "$media_type" "$encoded"
}

post_embedding() {
    local label="$1"
    local payload="$2"
    local body_file
    local status

    body_file="$(mktemp)"
    status="$(curl -sS -o "$body_file" -w "%{http_code}" \
        -X POST "${BASE_URL%/}/embeddings" \
        -H "Authorization: Bearer ${SILICONFLOW_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$payload")"

    echo ""
    echo "== ${label} =="
    echo "HTTP ${status}"
    if [ "$status" -ge 200 ] && [ "$status" -lt 300 ] && grep -q '"embedding"' "$body_file"; then
        echo "PASS"
        rm -f "$body_file"
        return 0
    fi

    echo "FAIL"
    sed -n '1,20p' "$body_file" | sed -E 's/("embedding"[[:space:]]*:[[:space:]]*)\[[^]]*/\1[.../g'
    rm -f "$body_file"
    return 1
}

post_embedding_with_image_tokens() {
    local label="$1"
    local payload="$2"
    local body_file
    local http_status
    local image_tokens

    body_file="$(mktemp)"
    http_status="$(curl -sS -o "$body_file" -w "%{http_code}" \
        -X POST "${BASE_URL%/}/embeddings" \
        -H "Authorization: Bearer ${SILICONFLOW_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "$payload")"

    echo ""
    echo "== ${label} =="
    echo "HTTP ${http_status}"
    if [ "$http_status" -ge 200 ] && [ "$http_status" -lt 300 ] && grep -q '"embedding"' "$body_file"; then
        image_tokens="$(grep -o '"image_tokens":[0-9]*' "$body_file" | head -1 | cut -d: -f2)"
        echo "image_tokens=${image_tokens:-missing}"
        if [ "${image_tokens:-0}" -gt 0 ]; then
            echo "PASS"
            rm -f "$body_file"
            return 0
        fi
    fi

    echo "FAIL"
    sed -n '1,20p' "$body_file" | sed -E 's/("embedding"[[:space:]]*:[[:space:]]*)\[[^]]*/\1[.../g'
    rm -f "$body_file"
    return 1
}

model_list_contains() {
    local body_file
    local http_status

    body_file="$(mktemp)"
    http_status="$(curl -sS -o "$body_file" -w "%{http_code}" \
        -H "Authorization: Bearer ${SILICONFLOW_API_KEY}" \
        "${BASE_URL%/}/models")"

    echo ""
    echo "== model list contains target model =="
    echo "HTTP ${http_status}"
    if [ "$http_status" -ge 200 ] && [ "$http_status" -lt 300 ] && grep -q "\"id\":\"${EMBEDDING_MODEL}\"" "$body_file"; then
        echo "PASS"
        rm -f "$body_file"
        return 0
    fi

    echo "FAIL"
    sed -n '1,20p' "$body_file"
    rm -f "$body_file"
    return 1
}

text_string_payload="$(embedding_payload "$EMBEDDING_MODEL" "\"${QUERY_TEXT}\"" "$EMBEDDING_DIMENSIONS")"
text_object_payload="$(embedding_payload "$EMBEDDING_MODEL" "{\"text\":\"${QUERY_TEXT}\"}" "$EMBEDDING_DIMENSIONS")"
batch_text_object_payload="$(embedding_payload "$EMBEDDING_MODEL" "[{\"text\":\"开心猫\"},{\"text\":\"无语表情\"}]" "$EMBEDDING_DIMENSIONS")"
image_uri="$(image_data_uri "$TEST_IMAGE_URL")"
if [ -z "$image_uri" ]; then
    echo "Failed to download TEST_IMAGE_URL for image embedding check." >&2
    exit 2
fi
image_data_uri_payload="$(embedding_payload "$EMBEDDING_MODEL" "{\"image\":\"${image_uri}\"}" "$EMBEDDING_DIMENSIONS")"

failed=0

if ! model_list_contains; then
    echo "Target embedding model was not found in SiliconFlow model list."
    failed=1
fi

if ! post_embedding "target model with text string input" "$text_string_payload"; then
    echo "Text string input failed for target model."
    failed=1
fi

if ! post_embedding "target model with current text object input" "$text_object_payload"; then
    echo "Current text object input failed for target model."
    failed=1
fi

if ! post_embedding "target model with current batch text object input" "$batch_text_object_payload"; then
    echo "Current batch text object input failed for target model."
    failed=1
fi

if ! post_embedding_with_image_tokens "target model with current image data URI input" "$image_data_uri_payload"; then
    echo "Current image data URI input failed or did not consume image tokens."
    failed=1
fi

exit "$failed"
