# Mimir Load Test Bottleneck Analyzer

An MCP (Model Context Protocol) server that gives LLMs direct query access to an Amazon Managed Prometheus (AMP) instance containing Mimir metrics. The LLM uses the exposed tools to iteratively query metrics, form hypotheses about bottlenecks, drill down into specific components, and produce a structured diagnostic report — all within a single conversation.

## How It Works

This is **not** a tool you use directly. It is an MCP server that an LLM (like Claude) connects to and drives autonomously. You configure it, point your LLM client at it, describe the issue you're investigating, and the LLM does the rest.

```
You describe the problem
        │
        ▼
Claude (via Claude Code / claude.ai)
        │
        │ MCP tool calls (automatic)
        ▼
  mimir-analyzer (this server)
        │
        │ HTTP + SigV4 auth
        ▼
  Amazon Managed Prometheus (AMP)
```

The `--help` output, tool descriptions, investigation approach, and example queries embedded in this server are all written **for the LLM** — they teach it how to systematically investigate Mimir issues. When Claude Code first connects to the server, it reads this information and uses it to guide its investigation.

## Getting Started

### 1. Build

```bash
go build -o mimir-analyzer .
```

### 2. Configure

```bash
# Required
export AMP_ENDPOINT=https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxxxx
export AWS_REGION=us-east-1
export LOADTEST_START=2024-01-15T10:00:00Z   # Start of the time window to investigate
export LOADTEST_END=2024-01-15T12:00:00Z     # End of the time window to investigate

# Optional
export QUERY_TIMEOUT_SECONDS=30       # default 30
export MAX_SERIES_RETURNED=2000       # default 2000
```

AWS credentials must already be present in your environment (via env vars, `~/.aws/credentials`, or an instance profile).

### 3. Register with Claude Code

Add to your `.claude/mcp.json`:

```json
{
  "mcpServers": {
    "mimir-analyzer": {
      "command": "/path/to/mimir-analyzer",
      "env": {
        "AMP_ENDPOINT": "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxxxx",
        "AWS_REGION": "us-east-1",
        "LOADTEST_START": "2024-01-15T10:00:00Z",
        "LOADTEST_END": "2024-01-15T12:00:00Z"
      }
    }
  }
}
```

### 4. Prompt the LLM

Once connected, describe the issue you want investigated. Examples:

> "Our Mimir load test with 261k alerts is showing missed rule evaluations. Use the mimir-analyzer tools to investigate which component is the bottleneck and produce a diagnostic report."

> "Query latency spiked during our load test between 10:00 and 11:30. Investigate what caused the latency increase across Mimir subsystems."

> "Run diagnostics across all Mimir subsystems for this load test window and flag any anomalies."

The LLM will autonomously call the MCP tools to discover metrics, run diagnostic bundles, drill down with targeted queries, correlate timing, and produce a structured report.

## MCP Tools (used by the LLM)

These tools are called by the LLM, not by humans directly:

| Tool | Description |
|------|-------------|
| `query_instant` | Run a PromQL instant query, clamped to the configured time window |
| `query_range` | Run a PromQL range query for trend analysis and correlation |
| `list_metrics` | Discover available metric names scoped to the time window |
| `run_diagnostic_bundle` | Run pre-built diagnostic queries for a Mimir subsystem |

All queries are automatically clamped to the `[LOADTEST_START, LOADTEST_END]` window — the LLM cannot accidentally query outside the configured range.

## Diagnostic Bundles

Pre-built query sets are available for each Mimir subsystem:

- **ruler** — Rule evaluation latency, missed evaluations, ring health (8 queries)
- **ingester** — Ingestion rate, active series, WAL/flush latency (5 queries)
- **querier** — Query latency, queue depth, errors, retries (4 queries)
- **distributor** — Receive latency, push/replication failures (3 queries)
- **compactor** — Compaction runs and failures (2 queries)
- **store_gateway** — Series fetch latency, cache hit rate (2 queries)
- **all** — Runs every subsystem bundle

## Run Tests

```bash
go test ./...           # All tests
go test -race ./...     # With race detector
```

## Project Structure

```
├── main.go              # Entrypoint, --help (LLM-facing), wiring
├── config/              # Environment variable parsing, time window clamping
├── amp/                 # AMP HTTP client with SigV4 signing
├── diagnostics/         # Pre-built query bundles per Mimir subsystem
├── mcp/                 # MCP server, tool definitions, handlers
└── report/              # Markdown report formatter
```
