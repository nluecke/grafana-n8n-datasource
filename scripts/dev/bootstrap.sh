#!/usr/bin/env bash
# Prepares the local n8n for development:
#   1. waits until n8n is ready
#   2. creates the owner on a fresh instance (N8N_OWNER_EMAIL / N8N_OWNER_PASSWORD)
#   3. creates a read-only API key for Grafana (N8N_API_KEY) and a key for seeding
#      (N8N_SEED_API_KEY) unless .env already contains working keys
#   4. imports the seed workflows unless they exist
# Uses n8n's internal REST API for steps 2-3; keys are written to .env, never printed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT/.env}"
[ -f "$ENV_FILE" ] || cp "$ROOT/.env.example" "$ENV_FILE"
set -a; . "$ENV_FILE"; set +a

N8N_BASE="${N8N_BASE:-http://localhost:${N8N_PORT:-5678}}"
OWNER_EMAIL="${N8N_OWNER_EMAIL:-admin@example.com}"
OWNER_PASSWORD="${N8N_OWNER_PASSWORD:-DevPassw0rd!}"
READ_SCOPES='["workflow:list","workflow:read","execution:list","execution:read","tag:list","project:list","insights:read","securityAudit:generate"]'
SEED_SCOPES='["workflow:list","workflow:read","workflow:create","workflow:activate"]'

log() { echo "bootstrap: $*"; }

set_env() { # set_env NAME VALUE  (replace or append in .env without printing the value)
  python3 -I - "$ENV_FILE" "$1" "$2" <<'PY'
import re, sys
path, name, value = sys.argv[1:4]
text = open(path).read()
line = f"{name}={value}"
if re.search(rf"^{name}=.*$", text, flags=re.M):
    text = re.sub(rf"^{name}=.*$", lambda _: line, text, flags=re.M)
else:
    text = text.rstrip("\n") + "\n" + line + "\n"
open(path, "w").write(text)
PY
}

key_works() { # key_works KEY → 0 if the public API accepts it
  [ -n "$1" ] && [ "$(curl -s -o /dev/null -w '%{http_code}' -H "X-N8N-API-KEY: $1" "$N8N_BASE/api/v1/workflows?limit=1")" = 200 ]
}

log "waiting for n8n at $N8N_BASE"
for _ in $(seq 1 90); do
  if curl -sf "$N8N_BASE/rest/settings" | python3 -I -c 'import json,sys; json.load(sys.stdin)' 2>/dev/null; then break; fi
  sleep 2
done
curl -sf "$N8N_BASE/rest/settings" >/dev/null || { log "n8n did not become ready"; exit 1; }

if key_works "${N8N_API_KEY:-}" && key_works "${N8N_SEED_API_KEY:-${N8N_API_KEY:-}}"; then
  log "API keys from .env work"
else
  JAR="$(mktemp)"; trap 'rm -f "$JAR"' EXIT
  needs_setup=$(curl -s "$N8N_BASE/rest/settings" | python3 -I -c 'import json,sys; print(json.load(sys.stdin)["data"]["userManagement"].get("showSetupOnFirstLoad", False))')
  if [ "$needs_setup" = "True" ]; then
    log "creating owner $OWNER_EMAIL"
    curl -sf -o /dev/null -X POST -H 'Content-Type: application/json' \
      -d "$(python3 -I -c 'import json,sys; print(json.dumps({"email":sys.argv[1],"firstName":"Dev","lastName":"Admin","password":sys.argv[2]}))' "$OWNER_EMAIL" "$OWNER_PASSWORD")" \
      "$N8N_BASE/rest/owner/setup"
  fi
  log "logging in as $OWNER_EMAIL"
  if ! curl -sf -o /dev/null -c "$JAR" -X POST -H 'Content-Type: application/json' \
    -d "$(python3 -I -c 'import json,sys; print(json.dumps({"emailOrLdapLoginId":sys.argv[1],"password":sys.argv[2]}))' "$OWNER_EMAIL" "$OWNER_PASSWORD")" \
    "$N8N_BASE/rest/login"; then
    log "login failed. Set N8N_OWNER_EMAIL / N8N_OWNER_PASSWORD in .env or put working keys into N8N_API_KEY / N8N_SEED_API_KEY."
    exit 1
  fi
  create_key() { # create_key LABEL SCOPES → prints key
    curl -sf -b "$JAR" -X POST -H 'Content-Type: application/json' \
      -d "{\"label\":\"$1\",\"expiresAt\":null,\"scopes\":$2}" "$N8N_BASE/rest/api-keys" |
      python3 -I -c 'import json,sys; print(json.load(sys.stdin)["data"]["rawApiKey"])'
  }
  set_env N8N_API_KEY "$(create_key "grafana (read-only) $(date +%s)" "$READ_SCOPES")"
  set_env N8N_SEED_API_KEY "$(create_key "seed $(date +%s)" "$SEED_SCOPES")"
  log "created API keys and stored them in .env"
  set -a; . "$ENV_FILE"; set +a
fi

SEED_KEY="${N8N_SEED_API_KEY:-$N8N_API_KEY}"
existing=$(curl -s -H "X-N8N-API-KEY: $SEED_KEY" "$N8N_BASE/api/v1/workflows?limit=250" |
  python3 -I -c 'import json,sys; print(sum(1 for w in json.load(sys.stdin)["data"] if w["name"].startswith("seed:")))')
if [ "$existing" -gt 0 ]; then
  log "seed workflows already present ($existing)"
else
  log "importing seed workflows"
  API_KEY_OVERRIDE="$SEED_KEY" N8N_URL="$N8N_BASE/api/v1" TARGET=local "$ROOT/scripts/probe/import-seed.sh"
fi
