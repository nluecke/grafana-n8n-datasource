#!/usr/bin/env bash
# Import seed workflows and activate them. TARGET=local|remote
# EVERY_MINUTES overrides the schedule interval (slow workflow uses twice the value).
. "$(dirname "$0")/lib.sh"
for f in "$ROOT"/seed/workflows/*.json; do
  body=$(python3 -I - "$f" "${EVERY_MINUTES:-}" <<'PY'
import json, sys
wf = json.load(open(sys.argv[1]))
if sys.argv[2]:
    for n in wf["nodes"]:
        if n["type"].endswith("scheduleTrigger"):
            m = int(sys.argv[2]) * (2 if "slow" in wf["name"] else 1)
            n["parameters"]["rule"]["interval"][0]["minutesInterval"] = m
print(json.dumps(wf))
PY
)
  id=$(api -X POST -H 'Content-Type: application/json' --data "$body" "$BASE/workflows" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["id"])')
  code=$(api -X POST -o /dev/null -w '%{http_code}' "$BASE/workflows/$id/activate")
  echo "$(basename "$f") -> $id (activate $code)"
done
