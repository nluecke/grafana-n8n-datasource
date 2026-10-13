#!/usr/bin/env bash
# Shared helpers. Usage: TARGET=local|remote. Never echo the API key.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_url_override="${N8N_URL:-}"
set -a; . "${ENV_FILE:-$ROOT/.env}"; set +a
N8N_URL="${_url_override:-$N8N_URL}"
case "${TARGET:-local}" in
  local)  BASE="$N8N_URL";       KEY="$N8N_API_KEY" ;;
  remote) BASE="$N8N_CLOUD_URL"; KEY="$N8N_CLOUD_API_KEY" ;;
  *) echo "unknown TARGET" >&2; exit 1 ;;
esac
# API_KEY_OVERRIDE lets callers use a different key than the one in .env (e.g. the seed key).
KEY="${API_KEY_OVERRIDE:-$KEY}"
[ -n "$KEY" ] || { echo "API key for $TARGET missing in .env" >&2; exit 1; }
CURL=(curl -sk -H "X-N8N-API-KEY: $KEY" -H "Accept: application/json")
api() { "${CURL[@]}" "$@"; }
