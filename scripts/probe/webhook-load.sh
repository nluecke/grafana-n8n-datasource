#!/usr/bin/env bash
# Fire N webhook calls against the seed webhook. Usage: TARGET=local N=5000 P=10 webhook-load.sh
. "$(dirname "$0")/lib.sh"
URL="${BASE%/api/v1}/webhook/seed-load"
seq "${N:-1000}" | xargs -P "${P:-10}" -I{} curl -sk -o /dev/null -w '%{http_code}\n' "$URL?i={}" | sort | uniq -c
