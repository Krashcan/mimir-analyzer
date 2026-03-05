# mimir-analyzer: Feedback from Debugging Session

## Context

Attempted to use `mimir-analyzer` (MCP server) to debug ruler evaluation failures during a load test between `2026-03-05T10:42:00Z` and `2026-03-05T15:30:00Z` 

## What Worked

- `list_metrics` successfully discovered metrics from AMP.
- `run_diagnostic_bundle subsystem=ruler` connected and returned results for some queries.
- Time window clamping behaved correctly.
- SigV4 auth worked fine once endpoint was correct.

## What Didn't Work / Friction Points

### 1. `check_connection` always fails against AMP (Bug)

`CheckConnection` hits `/api/v1/status/buildinfo`, which AMP does **not** implement. AMP returns 404 for this path. The tool then reports `wrong_endpoint` even though the endpoint is perfectly valid.

**Impact:** Following the documented workflow ("call check_connection first") always produces a misleading failure, wasting time diagnosing a non-existent endpoint problem.

**Fix:** Use an endpoint AMP actually supports for the health check, e.g., `/api/v1/labels` with a short time window, or `/api/v1/query?query=up&time=...`.

### 2. Ruler diagnostic bundle uses wrong metric names (Bug)

The ruler bundle in `diagnostics/ruler.go` uses metric names that don't exist in a standard Mimir metamonitoring setup:

| Bundle uses (doesn't exist)                                | Actual metric name in AMP                                   |
|------------------------------------------------------------|-------------------------------------------------------------|
| `cortex_ruler_rule_evaluation_duration_seconds_bucket`     | `cortex_prometheus_rule_evaluation_duration_seconds`         |
| `cortex_ruler_rule_evaluation_missed_iterations_total`     | `cortex_prometheus_rule_group_iterations_missed_total`       |
| `cortex_ruler_rule_group_rules`                            | `cortex_prometheus_rule_group_rules`                         |
| `cortex_ruler_rule_group_duration_seconds_bucket`          | `cortex_prometheus_rule_group_duration_seconds`              |
| `cortex_ruler_rule_evaluations_total`                      | `cortex_prometheus_rule_evaluations_total`                   |

4 out of 8 ruler bundle queries returned `empty` because of this. The `--help` text and `CLAUDE.md` also reference the wrong metric names.

**Fix:** Update `diagnostics/ruler.go` queries to use the correct `cortex_prometheus_rule_*` metric names. Also note that the actual missed evaluations metric is `cortex_prometheus_rule_group_iterations_missed_total`, not `cortex_ruler_rule_evaluation_missed_iterations_total`.

### 3. No CLI mode -- MCP-only is painful for non-MCP clients (Feature Request)

The tool only works as an MCP server (stdio JSON-RPC). Using it without an MCP client required writing a Python wrapper to construct JSON-RPC messages, parse nested JSON responses, and pipe them through stdin/stdout. This is extremely cumbersome for:

- Quick ad-hoc queries during debugging
- CI/CD pipelines
- Users who don't have Claude Code / an MCP client set up

**Suggestion:** Add a CLI subcommand mode alongside MCP:

```
# CLI mode
mimir-analyzer query "sum(rate(cortex_ruler_queries_failed_total[5m]))"
mimir-analyzer query-range "rate(...[5m])" --step 5m
mimir-analyzer list-metrics --match '{__name__=~"cortex_ruler.*"}'
mimir-analyzer diagnose ruler
mimir-analyzer diagnose all --format summary

# MCP mode (current behavior, keep as default)
mimir-analyzer
```

This keeps MCP as the primary mode but makes the tool usable standalone.

### 4. Diagnostic bundle output is too verbose / no summarization (Feature Request)

`run_diagnostic_bundle subsystem=ruler` returned ~286,000 lines of raw JSON time series data. Every data point from every series across the full 5-hour window at 1m step was included.

For an LLM consumer, this blows up the context window. For a human, it's unreadable.

**Suggestion:** Add a summary layer to the diagnostic bundle output. For each query in the bundle, compute and return:

```json
{
  "name": "querier_errors_from_ruler",
  "query": "rate(cortex_ruler_queries_failed_total[5m])",
  "summary": {
    "series_count": 62,
    "max_value": 41.40,
    "max_value_timestamp": "2026-03-05T14:15:00Z",
    "avg_value": 3.21,
    "non_zero_percentage": 78.5,
    "trend": "increasing"
  },
  "status": "warning"
}
```

Return the full raw data only when explicitly requested (e.g., `--verbose` flag or a separate `query_range` call). The summary should be the default output for `run_diagnostic_bundle`.

### 5. `--help` and CLAUDE.md reference wrong `AMP_ENDPOINT` format inconsistently

The `--help` example shows:
```
e.g. https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxxxx
```

But `CLAUDE.md` Configuration section also shows the correct format. Meanwhile, the `mcp.json` we initially had used just the base URL without workspace path (`https://aps-workspaces.ap-southeast-2.amazonaws.com`), which caused 404 errors on every call.

**Fix:** Validate the `AMP_ENDPOINT` format at startup. If it doesn't match the pattern `https://aps-workspaces.*.amazonaws.com/workspaces/ws-*`, log a clear warning. This catches misconfiguration before any queries are attempted.

### 6. Diagnostic bundle should auto-discover metric names (Feature Request)

Since different Mimir versions and metamonitoring setups use different metric prefixes (`cortex_ruler_rule_*` vs `cortex_prometheus_rule_*`), the bundle should do a `list_metrics` call first to discover which variants exist, then use the correct names.

This would make the tool work across different Mimir setups without needing to hardcode metric names.

## Summary of Recommended Changes

| Priority | Type    | Issue                                           |
|----------|---------|-------------------------------------------------|
| P0       | Bug     | `check_connection` uses unsupported AMP endpoint |
| P0       | Bug     | Ruler bundle uses wrong metric names             |
| P1       | Feature | Add CLI mode alongside MCP                       |
| P1       | Feature | Summarize diagnostic bundle output               |
| P2       | Feature | Validate `AMP_ENDPOINT` format at startup        |
| P2       | Feature | Auto-discover metric name variants               |