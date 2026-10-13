#!/usr/bin/env python3
"""Generates src/dashboards/n8n-workflow-executions.json.

Layout follows the n8n community dashboards (grafana.com 24474 / 24475):
rows, 3-high stat tiles with background colour, then trends and tables.
Run: python3 scripts/dev/gen-dashboard.py
"""
import json
import pathlib

DS = {"type": "nluecke-n8n-datasource", "uid": "${datasource}"}
OUT = pathlib.Path(__file__).resolve().parents[2] / "src" / "dashboards" / "n8n-workflow-executions.json"

STATUS_COLORS = {
    "success": "green",
    "error": "red",
    "crashed": "dark-red",
    "canceled": "gray",
    "running": "blue",
    "waiting": "yellow",
    "new": "purple",
    "unknown": "text",
}

_next_id = 0


def pid():
    global _next_id
    _next_id += 1
    return _next_id


def q(ref="A", **kw):
    base = {"refId": ref, "datasource": DS, "workflowIds": ["$workflow"]}
    base.update(kw)
    return base


def row(title, y):
    return {"type": "row", "id": pid(), "title": title, "collapsed": False,
            "gridPos": {"h": 1, "w": 24, "x": 0, "y": y}, "panels": []}


def stat(title, x, y, w, targets, unit="short", thresholds=None, color_mode="background",
         description="", decimals=None, fields=""):
    steps = thresholds or [{"color": "blue", "value": None}]
    defaults = {"unit": unit, "color": {"mode": "thresholds"},
                "thresholds": {"mode": "absolute", "steps": steps}}
    if decimals is not None:
        defaults["decimals"] = decimals
    return {
        "type": "stat", "id": pid(), "title": title, "description": description, "datasource": DS,
        "gridPos": {"h": 3, "w": w, "x": x, "y": y},
        "targets": targets,
        "fieldConfig": {"defaults": defaults, "overrides": []},
        "options": {"colorMode": color_mode, "graphMode": "none", "justifyMode": "auto",
                    "textMode": "auto", "wideLayout": True, "showPercentChange": False,
                    "reduceOptions": {"calcs": ["lastNotNull"], "fields": fields, "values": False}},
    }


def status_overrides():
    return [{"matcher": {"id": "byName", "options": status},
             "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": color}}]}
            for status, color in STATUS_COLORS.items()]


def timeseries(title, x, y, w, h, targets, unit="short", draw="line", stack=False, overrides=None,
               description=""):
    return {
        "type": "timeseries", "id": pid(), "title": title, "description": description, "datasource": DS,
        "gridPos": {"h": h, "w": w, "x": x, "y": y},
        "targets": targets,
        "fieldConfig": {
            "defaults": {
                "unit": unit, "color": {"mode": "palette-classic"},
                "custom": {"drawStyle": draw, "fillOpacity": 80 if draw == "bars" else 0, "lineWidth": 1,
                           "stacking": {"mode": "normal" if stack else "none", "group": "A"},
                           "spanNulls": False, "showPoints": "auto"},
                **({"min": 0, "max": 1} if unit == "percentunit" else {}),
            },
            "overrides": overrides or [],
        },
        "options": {"legend": {"displayMode": "list", "placement": "bottom", "showLegend": True},
                    "tooltip": {"mode": "multi", "sort": "desc"}},
    }


def table(title, x, y, w, h, targets, exclude=(), transformations=None, overrides=None, description=""):
    trans = list(transformations or [])
    if exclude:
        trans.append({"id": "organize", "options": {"excludeByName": {name: True for name in exclude}}})
    return {
        "type": "table", "id": pid(), "title": title, "description": description, "datasource": DS,
        "gridPos": {"h": h, "w": w, "x": x, "y": y},
        "targets": targets,
        "transformations": trans,
        "fieldConfig": {"defaults": {"custom": {"align": "auto", "filterable": True}}, "overrides": overrides or []},
        "options": {"showHeader": True, "cellHeight": "sm", "footer": {"show": False}},
    }


def status_cell_override():
    return {"matcher": {"id": "byName", "options": "status"},
            "properties": [
                {"id": "custom.cellOptions", "value": {"type": "color-text"}},
                {"id": "mappings", "value": [{"type": "value", "options": {
                    s: {"color": c, "index": i} for i, (s, c) in enumerate(STATUS_COLORS.items())}}]},
            ]}


panels = []
y = 0

panels.append(row("Executive summary", y)); y += 1
rng = {"queryType": "executionStats", "bucket": "range", "groupBy": "none"}
panels += [
    stat("Executions", 0, y, 4, [q(metric="count", **rng)],
         description="Executions started in the time range (from the execution list)."),
    stat("Success rate", 4, y, 4, [q(metric="successRate", **rng)], unit="percentunit", decimals=1,
         thresholds=[{"color": "red", "value": None}, {"color": "orange", "value": 0.9}, {"color": "green", "value": 0.98}],
         description="success / (success + error + crashed). Running and canceled executions are not counted."),
    stat("Failed executions", 8, y, 4, [q(metric="count", statuses=["error", "crashed"], **rng)],
         thresholds=[{"color": "green", "value": None}, {"color": "red", "value": 1}]),
    stat("p90 duration", 12, y, 4, [q(metric="duration", percentiles=[90], **rng)], unit="ms",
         thresholds=[{"color": "blue", "value": None}]),
    stat("Running now", 16, y, 4, [q(metric="count", statuses=["running", "waiting"], **rng)],
         thresholds=[{"color": "text", "value": None}, {"color": "blue", "value": 1}]),
    stat("Active workflows", 20, y, 4,
         [{"refId": "A", "datasource": DS, "queryType": "workflows", "workflowIds": ["$workflow"]}],
         description="Workflows that are published/active."),
]
# Active workflows: count rows where active == true.
panels[-1]["transformations"] = [
    {"id": "filterByValue", "options": {"type": "include", "match": "any", "filters": [
        {"fieldName": "active", "config": {"id": "equal", "options": {"value": True}}}]}},
    {"id": "reduce", "options": {"reducers": ["count"], "fields": "", "mode": "reduceFields", "includeTimeField": False}},
]
panels[-1]["options"]["reduceOptions"]["fields"] = "/^id$/"
y += 3

panels.append(row("Execution trends", y)); y += 1
panels += [
    timeseries("Executions by status", 0, y, 12, 9,
               [q(queryType="executionStats", metric="count", groupBy="status")],
               draw="bars", stack=True, overrides=status_overrides()),
    timeseries("Execution duration percentiles", 12, y, 12, 9,
               [q(queryType="executionStats", metric="duration", groupBy="none", percentiles=[50, 90, 99])],
               unit="ms"),
]
y += 9

panels.append(row("Workflow quality", y)); y += 1
panels += [
    table("Top failing workflows", 0, y, 12, 9,
          [q(queryType="executionStats", metric="count", groupBy="workflow", statuses=["error", "crashed"], bucket="range")],
          transformations=[
              {"id": "reduce", "options": {"reducers": ["lastNotNull"], "mode": "seriesToRows", "includeTimeField": False}},
              {"id": "filterByValue", "options": {"type": "exclude", "match": "any", "filters": [
                  {"fieldName": "Last *", "config": {"id": "lowerOrEqual", "options": {"value": 0}}}]}},
              {"id": "sortBy", "options": {"sort": [{"field": "Last *", "desc": True}]}},
              {"id": "limit", "options": {"limitField": 10}},
              {"id": "organize", "options": {"renameByName": {"Field": "Workflow", "Last *": "Failed executions"}}},
          ],
          overrides=[{"matcher": {"id": "byName", "options": "Failed executions"}, "properties": [
              {"id": "custom.cellOptions", "value": {"type": "gauge", "mode": "basic"}},
              {"id": "color", "value": {"mode": "fixed", "fixedColor": "red"}}]}]),
    timeseries("Error rate by workflow", 12, y, 12, 9,
               [q(queryType="executionStats", metric="errorRate", groupBy="workflow")],
               unit="percentunit", description="Buckets without finished executions are left empty."),
]
y += 9

panels.append(row("Recent failures", y)); y += 1
panels.append(table("Failed executions", 0, y, 24, 9,
                    [q(queryType="executions", statuses=["error", "crashed"])],
                    exclude=("workflowId", "url", "mode"),
                    overrides=[status_cell_override()],
                    description="Click an execution id to open it in n8n."))
y += 9

panels.append(row("Long term (n8n Insights, last 7 days)", y)); y += 1
ins = stat("Insights summary", 0, y, 24, [{"refId": "A", "datasource": DS, "queryType": "insights", "insightsMode": "summary"}],
           color_mode="value",
           description="Instance-wide aggregates from n8n Insights. Not affected by execution pruning; updated hourly.")
ins["gridPos"]["h"] = 4
# Show the values only; the *Deviation fields compare with the previous period.
ins["transformations"] = [{"id": "organize", "options": {"excludeByName": {
    f"{n}Deviation": True for n in ("total", "failed", "failureRate", "averageRunTime", "timeSaved")}}}]
ins["timeFrom"] = "7d"
ins["fieldConfig"]["overrides"] = [
    {"matcher": {"id": "byName", "options": n}, "properties": [{"id": "displayName", "value": d}]}
    for n, d in [("total", "Executions"), ("failed", "Failed"), ("failureRate", "Failure rate"),
                 ("averageRunTime", "Avg run time"), ("timeSaved", "Time saved")]]
panels.append(ins)
y += 4
ts = timeseries("Executions per day (Insights)", 0, y, 24, 8,
                [{"refId": "A", "datasource": DS, "queryType": "insights", "insightsMode": "timeseries", "insightsBucket": "1d"}],
                draw="bars")
ts["timeFrom"] = "7d"
ts["fieldConfig"]["overrides"] = [
    {"matcher": {"id": "byRegexp", "options": "^(failureRate|averageRunTime|timeSaved)$"},
     "properties": [{"id": "custom.hideFrom", "value": {"legend": True, "tooltip": False, "viz": True}}]},
    {"matcher": {"id": "byName", "options": "failed"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "red"}}]},
    {"matcher": {"id": "byName", "options": "total"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "green"}}]},
]
panels.append(ts)
y += 8

panels.append(row("Workflows", y)); y += 1
panels.append(table("Workflows", 0, y, 24, 8,
                    [{"refId": "A", "datasource": DS, "queryType": "workflows", "workflowIds": ["$workflow"]}],
                    exclude=("url", "projectId"),
                    description="Click a workflow name to open it in n8n. 'skipsSuccessfulExecutions' marks workflows whose success rate cannot be computed."))

dashboard = {
    "uid": "n8n-workflow-executions-api",
    "title": "n8n Workflow Executions (API)",
    "description": "Workflow and execution observability for n8n Cloud and self-hosted n8n via the n8n public API.",
    "tags": ["n8n", "api"],
    "editable": True,
    "graphTooltip": 1,
    "time": {"from": "now-24h", "to": "now"},
    "refresh": "1m",
    "schemaVersion": 41,
    "version": 1,
    "links": [],
    "annotations": {"list": []},
    "templating": {"list": [
        {"name": "datasource", "label": "n8n", "type": "datasource", "query": "nluecke-n8n-datasource",
         "current": {}, "hide": 0, "refresh": 1},
        {"name": "workflow", "label": "Workflow", "type": "query", "datasource": DS,
         "query": {"refId": "workflow", "resource": "workflows"}, "definition": "workflows",
         "multi": True, "includeAll": True, "allValue": "$__all", "current": {"text": "All", "value": "$__all"},
         "refresh": 1, "sort": 1, "hide": 0},
    ]},
    "panels": panels,
}

OUT.parent.mkdir(parents=True, exist_ok=True)
OUT.write_text(json.dumps(dashboard, indent=2) + "\n")
print(f"wrote {OUT.relative_to(OUT.parents[2])} with {len(panels)} panels")
