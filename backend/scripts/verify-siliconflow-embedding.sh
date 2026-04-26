#!/usr/bin/env bash

# Verify SiliconFlow embeddings model availability and input payload shape.
#
# Required:
#   SILICONFLOW_API_KEY=... (exported or defined in backend/.env)
#
# Optional:
#   ENV_FILE=/path/to/.env
#   SILICONFLOW_BASE_URL=https://api.siliconflow.cn/v1
#   CONFIG_MODEL=Qwen/Qwen3-VL-Embedding-8B
#   CONFIG_DIMENSIONS=1024
#   CONTROL_MODEL=Qwen/Qwen3-Embedding-8B
#   CONTROL_DIMENSIONS=
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
CONFIG_MODEL="${CONFIG_MODEL:-Qwen/Qwen3-VL-Embedding-8B}"
CONFIG_DIMENSIONS="${CONFIG_DIMENSIONS:-1024}"
CONTROL_MODEL="${CONTROL_MODEL:-Qwen/Qwen3-Embedding-8B}"
CONTROL_DIMENSIONS="${CONTROL_DIMENSIONS:-}"
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
  CONFIG_MODEL          Model from config.yaml to validate, default: ${CONFIG_MODEL}
  CONFIG_DIMENSIONS     Dimensions for CONFIG_MODEL, default: ${CONFIG_DIMENSIONS}
  CONTROL_MODEL         Known text embedding model for payload-shape checks, default: ${CONTROL_MODEL}
  CONTROL_DIMENSIONS    Optional dimensions for CONTROL_MODEL
  TEST_IMAGE_URL        Public image URL for image object input checks

Checks:
  1. CONFIG_MODEL with string input: verifies whether the configured model is accepted.
  2. CONTROL_MODEL with string input: verifies the API/key baseline.
  3. CONTROL_MODEL with object input {"text": "..."}: verifies the current client payload shape.
  4. CONFIG_MODEL with object input {"image": "..."}: verifies the current image-ingest payload shape.
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
    sed -n '1,20p' "$body_file"
    rm -f "$body_file"
    return 1
}

config_string_payload="$(embedding_payload "$CONFIG_MODEL" "\"${QUERY_TEXT}\"" "$CONFIG_DIMENSIONS")"
control_string_payload="$(embedding_payload "$CONTROL_MODEL" "\"${QUERY_TEXT}\"" "$CONTROL_DIMENSIONS")"
control_object_payload="$(embedding_payload "$CONTROL_MODEL" "{\"text\":\"${QUERY_TEXT}\"}" "$CONTROL_DIMENSIONS")"
config_image_object_payload="$(embedding_payload "$CONFIG_MODEL" "{\"image\":\"${TEST_IMAGE_URL}\"}" "$CONFIG_DIMENSIONS")"

failed=0

if ! post_embedding "configured model with string input" "$config_string_payload"; then
    echo "Configured model was not accepted by SiliconFlow embeddings."
    failed=1
fi

if ! post_embedding "control model with string input" "$control_string_payload"; then
    echo "Control model failed; check API key, base URL, or CONTROL_MODEL before judging payload shape."
    failed=1
elif ! post_embedding "control model with object input used by current client" "$control_object_payload"; then
    echo "Object input was rejected; SiliconFlow text embeddings should be sent as a string/array payload."
    failed=1
else
    echo "Object input was accepted by the current SiliconFlow endpoint."
fi

if ! post_embedding "configured model with image object input used by current image ingest" "$config_image_object_payload"; then
    echo "Image object input was rejected; current image-mode SiliconFlow payload shape needs provider-specific validation/fixing."
    failed=1
else
    echo "Image object input was accepted by the current SiliconFlow endpoint."
fi

exit "$failed"
