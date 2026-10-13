#!/usr/bin/env bash
# Page through /executions (newest first) until startedAt < SINCE or MAX_PAGES.
# Usage: TARGET=local SINCE=2026-10-12T00:00:00Z LIMIT=250 [SERVER_FILTER=1] paginate.sh
. "$(dirname "$0")/lib.sh"
python3 -I - "$BASE" "${SINCE:?}" "${LIMIT:-250}" "${MAX_PAGES:-100}" "${SERVER_FILTER:-0}" "$KEY" <<'PY'
import json, ssl, sys, time, urllib.request, urllib.parse
base, since, limit, maxp, sf, key = sys.argv[1:7]
ctx = ssl._create_unverified_context()
cursor, pages, rows, t0, statuses = None, 0, 0, time.time(), {}
stopped_by = "end"
while pages < int(maxp):
    q = {"limit": limit}
    if cursor: q["cursor"] = cursor
    if sf == "1": q["startedAfter"] = since
    req = urllib.request.Request(f"{base}/executions?{urllib.parse.urlencode(q)}", headers={"X-N8N-API-KEY": key})
    t = time.time()
    try:
        body = json.load(urllib.request.urlopen(req, context=ctx))
    except urllib.error.HTTPError as e:
        print("HTTP", e.code, e.read()[:200]); sys.exit(1)
    pages += 1
    data = body["data"]
    old = [d for d in data if (d.get("startedAt") or "") < since]
    for d in data:
        if (d.get("startedAt") or "") >= since:
            rows += 1; statuses[d["status"]] = statuses.get(d["status"], 0) + 1
    print(f"page {pages}: {len(data)} rows in {time.time()-t:.3f}s")
    cursor = body.get("nextCursor")
    if old: stopped_by = "cutoff"; break
    if not cursor: break
else:
    stopped_by = "maxPages"
print(json.dumps({"pages": pages, "rows_in_window": rows, "total_s": round(time.time()-t0, 3), "stopped_by": stopped_by, "statuses": statuses}))
PY
