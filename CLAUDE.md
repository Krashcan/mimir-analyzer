# Mimir Load Test Bottleneck Analyzer

## Project Overview

This is a **Go MCP (Model Context Protocol) server** that gives Claude direct query access
to an Amazon Managed Prometheus (AMP) instance containing Mimir load test metrics.

Claude uses this tool to iteratively query metrics, form hypotheses about bottlenecks,
drill down into specific components, and produce a structured diagnostic report — all
within a single conversation.

**Problem being solved:** Mimir load tests can surface performance issues — missed
evaluations, high latency, backpressure — that are hard to diagnose manually. This
tool enables systematic investigation across all Mimir subsystems for any issue
the user describes.

---

## Architecture

```
Claude (claude.ai or Claude Code)
        │
        │ MCP tool calls
        ▼
  Go MCP Server (this repo)
        │
        │ HTTP + SigV4 auth
        ▼
  Amazon Managed Prometheus (AMP)
        │
        │ stores
        ▼
  Mimir load test metrics
```

The MCP server exposes tools that Claude calls to:
1. Run instant PromQL queries
2. Run range queries (for trend analysis)
3. List available metrics (label-based discovery)
4. Run pre-built diagnostic query bundles per Mimir subsystem

---

## Repository Structure

```
mimir-bottleneck-analyzer/
├── CLAUDE.md                  ← this file
├── README.md
├── go.mod
├── go.sum
├── main.go                    ← entrypoint, wires everything
├── mcp/
│   ├── server.go              ← MCP server (stdio transport)
│   ├── server_test.go
│   ├── tools.go               ← tool definitions + dispatch
│   ├── tools_test.go
│   └── types.go               ← shared MCP types
├── amp/
│   ├── client.go              ← AMP HTTP client with SigV4
│   ├── client_test.go
│   ├── query.go               ← instant + range query methods
│   ├── query_test.go
│   └── discover.go            ← metric/label discovery
│   └── discover_test.go
├── diagnostics/
│   ├── bundles.go             ← pre-built query bundles per subsystem
│   ├── bundles_test.go
│   ├── ruler.go               ← ruler-specific diagnostic queries
│   ├── ruler_test.go
│   ├── ingester.go
│   ├── ingester_test.go
│   ├── querier.go
│   ├── querier_test.go
│   ├── distributor.go
│   ├── distributor_test.go
│   ├── compactor.go
│   ├── compactor_test.go
│   └── store_gateway.go
│   └── store_gateway_test.go
├── report/
│   ├── formatter.go           ← formats findings into markdown report
│   └── formatter_test.go
└── config/
    ├── config.go              ← config from env vars
    └── config_test.go
```

---

## Development Methodology: Red/Green TDD

**All code in this project must be written using strict red/green TDD.** No
production code is written without a failing test first. This applies to every
package, every function, every bug fix.

### The cycle

```
1. RED   — write a failing test that describes the behaviour you want
           run: go test ./... → must see FAIL before proceeding
2. GREEN — write the minimum production code to make it pass
           run: go test ./... → must see PASS
3. REFACTOR — clean up, improve naming, remove duplication
           run: go test ./... → must still PASS
```

Never skip the RED step. If you write production code first and then write a test
that immediately passes, you have not done TDD — you have written a test that
proves nothing.

### What to test per package

**`config/`**
- Valid env vars parse correctly into `Config`
- Missing `AMP_ENDPOINT` or `AWS_REGION` returns an error
- Malformed `LOADTEST_START` / `LOADTEST_END` returns a descriptive error
- `LOADTEST_END` before `LOADTEST_START` returns an error
- `ClampToWindow` clamps start/end correctly across all edge cases:
    - both within window → unchanged
    - start before window → clamped to `LoadtestStart`
    - end after window → clamped to `LoadtestEnd`
    - both zero → returns full window

**`amp/`**
- `sigV4RoundTripper` signs requests correctly (use `httptest.Server` to capture headers)
- 401/403 responses produce a clear "credentials expired or invalid" error message
- `query_instant` serialises the PromQL expression and time into the correct URL params
- `query_range` serialises expr, start, end, step correctly
- `ClampToWindow` is called before every outbound request (verify via captured URL params)
- Empty result sets return `status: "empty"`, not an error
- AMP HTTP errors (5xx) are wrapped with context

**`diagnostics/`**
- Each subsystem bundle returns the expected set of PromQL expressions
- `run_diagnostic_bundle` with `subsystem=all` runs every bundle
- Optional `start`/`end` narrowing is applied correctly
- Narrowed range outside the window is clamped (not rejected)

**`mcp/`**
- `query_instant` tool handler rejects missing `expr`
- `query_range` tool handler defaults step to `1m` when omitted
- `list_metrics` tool handler defaults limit to 200
- `run_diagnostic_bundle` rejects unknown subsystem values
- All tool responses include the `"window"` field
- Tool handler errors produce valid MCP error responses (not panics)

**`report/`**
- Formatter produces valid markdown
- Empty findings produce a report with a clear "no data" section rather than blank output

### Test helpers and fakes

Do not make real HTTP calls in tests. Use fakes:

```go
// amp/fake_test.go — shared across amp package tests
func newFakeAMPServer(t *testing.T, responses map[string]string) *httptest.Server {
    t.Helper()
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        expr := r.URL.Query().Get("query")
        if resp, ok := responses[expr]; ok {
            w.Header().Set("Content-Type", "application/json")
            fmt.Fprint(w, resp)
            return
        }
        http.Error(w, "unexpected query: "+expr, http.StatusBadRequest)
    }))
}
```

For MCP tool handler tests, call the handler functions directly — do not go
through the stdio transport.

### Running tests

```bash
# Run all tests
go test ./...

# Run with race detector (required before any PR/merge)
go test -race ./...

# Run a specific package
go test ./amp/...

# Verbose output to see red/green clearly
go test -v ./...
```

### Definition of done for each piece of work

A function or feature is only considered complete when:
1. There is at least one test that was written **before** the production code
2. `go test -race ./...` passes with no failures or race conditions
3. The test names clearly describe the behaviour, not the implementation
    - Good: `TestClampToWindow_StartBeforeWindow`
    - Bad:  `TestClampToWindow_Case3`

---

## Configuration

All configuration via environment variables:

```bash
# Required
AMP_ENDPOINT=https://aps-workspaces.<region>.amazonaws.com/workspaces/<workspace-id>
AWS_REGION=us-east-1

# Load test window — all queries are automatically clamped to this range (RFC3339)
LOADTEST_START=2024-01-15T10:00:00Z
LOADTEST_END=2024-01-15T12:00:00Z

# Auth — no extra config needed (see Auth section below)

# Optional tuning
QUERY_TIMEOUT_SECONDS=30       # default 30
MAX_SERIES_RETURNED=2000       # cap series per query to avoid huge responses
```

---

## AWS Authentication

This project uses **no explicit credential config**. Auth works via the standard
AWS SDK default credential provider chain. Credentials must already be present
in the environment (e.g. via environment variables, `~/.aws/credentials`, or
an instance profile).

**Your workflow:**

```bash
# 1. Ensure AWS credentials are available in your environment

# 2. Start the MCP server with your load test window
AMP_ENDPOINT=https://... \
AWS_REGION=us-east-1 \
LOADTEST_START=2024-01-15T10:00:00Z \
LOADTEST_END=2024-01-15T12:00:00Z \
./mimir-analyzer
```

**In code, this is all that's needed:**

```go
// amp/client.go — SDK finds credentials via the default chain automatically
cfg, err := config.LoadDefaultConfig(context.TODO(),
    config.WithRegion(os.Getenv("AWS_REGION")),
)
if err != nil {
    log.Fatalf("failed to load AWS config: %v", err)
}
// Pass cfg.Credentials into the SigV4 round tripper — done
```

**Credential expiry:** If credentials expire mid-session, AMP will return a 403.
The MCP server must catch this and return a clear error rather than a cryptic
HTTP failure:

```go
// In the SigV4 round tripper's error handling:
if resp.StatusCode == 401 || resp.StatusCode == 403 {
    return nil, fmt.Errorf(
        "AWS credentials expired or invalid (HTTP %d) — ensure valid credentials are present in your environment",
        resp.StatusCode,
    )
}
```

This way Claude surfaces a clear, actionable message to you in the conversation
instead of a confusing query failure.

---

## MCP Tools Exposed to Claude

### `query_instant`
Run a PromQL instant query. Time is clamped to within `[LOADTEST_START, LOADTEST_END]`.

```json
{
  "name": "query_instant",
  "description": "Run a PromQL instant query against the AMP workspace. Time is automatically clamped to the configured load test window. Defaults to LOADTEST_END if no time given.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "expr": { "type": "string", "description": "PromQL expression" },
      "time": { "type": "string", "description": "RFC3339 timestamp within the load test window. Defaults to LOADTEST_END." }
    },
    "required": ["expr"]
  }
}
```

### `query_range`
Run a PromQL range query. `start` and `end` are clamped to `[LOADTEST_START, LOADTEST_END]`
— Claude cannot accidentally query outside the load test window.

```json
{
  "name": "query_range",
  "description": "Run a PromQL range query. start/end are clamped to the configured load test window. Defaults to the full window if not specified. Use step wisely — smaller steps = more data = slower.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "expr":  { "type": "string" },
      "start": { "type": "string", "description": "RFC3339. Clamped to LOADTEST_START if earlier. Defaults to LOADTEST_START." },
      "end":   { "type": "string", "description": "RFC3339. Clamped to LOADTEST_END if later. Defaults to LOADTEST_END." },
      "step":  { "type": "string", "description": "Duration like 15s, 1m, 5m. Defaults to 1m." }
    },
    "required": ["expr"]
  }
}
```

### `list_metrics`
Discover what metrics exist, filtered by a label matcher. Scoped to the load test window.

```json
{
  "name": "list_metrics",
  "description": "List metric names matching an optional label selector, scoped to the load test window.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "match": { "type": "string", "description": "Optional label selector, e.g. {job=~'mimir.*'}" },
      "limit": { "type": "integer", "description": "Max results. Default 200." }
    }
  }
}
```

### `run_diagnostic_bundle`
Run a pre-built set of queries for a specific Mimir subsystem. Always runs over the
full load test window unless `start`/`end` are explicitly narrowed within it.

```json
{
  "name": "run_diagnostic_bundle",
  "description": "Run pre-built diagnostic queries for a Mimir subsystem over the load test window. start/end can narrow the window but cannot exceed it.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "subsystem": {
        "type": "string",
        "enum": ["ruler", "ingester", "querier", "distributor", "compactor", "store_gateway", "query_frontend", "all"]
      },
      "start": { "type": "string", "description": "Optional narrowing start within the load test window." },
      "end":   { "type": "string", "description": "Optional narrowing end within the load test window." }
    },
    "required": ["subsystem"]
  }
}
```

---

## Diagnostic Query Bundles (Reference)

These are the queries Claude will use via `run_diagnostic_bundle`. They are also
runnable individually via `query_instant` / `query_range`.

### Ruler (primary suspect for missed evaluations)

```promql
# Rule evaluation lag — key signal
histogram_quantile(0.99, rate(cortex_ruler_rule_evaluation_duration_seconds_bucket[5m]))

# Missed evaluations — the metric we care about
increase(cortex_ruler_rule_evaluation_missed_iterations_total[5m])

# Rules per rule group (high counts = slow evaluation)
cortex_ruler_rule_group_rules

# Evaluation iteration duration
histogram_quantile(0.99, rate(cortex_ruler_rule_group_duration_seconds_bucket[5m]))

# Querier errors from ruler
rate(cortex_ruler_queries_failed_total[5m])

# Ruler ring status — check for unhealthy instances
cortex_ring_members{name="ruler"}

# Rule evaluations per second
rate(cortex_ruler_rule_evaluations_total[5m])

# Scheduler queue depth (if using rule sharding)
cortex_ruler_sync_rules_total
```

### Ingester

```promql
# Ingestion rate
rate(cortex_ingester_ingested_samples_total[5m])

# Ingester ring health
cortex_ring_members{name="ingester"}

# WAL replay / flush latency
histogram_quantile(0.99, rate(cortex_ingester_tsdb_head_truncation_duration_seconds_bucket[5m]))

# Active series per ingester
cortex_ingester_memory_series

# Chunk utilization
cortex_ingester_memory_chunks
```

### Querier / Query Frontend

```promql
# Query latency (ruler queries go through querier)
histogram_quantile(0.99, rate(cortex_querier_query_duration_seconds_bucket[5m]))

# Queue depth at query frontend
cortex_query_frontend_queue_length

# Querier errors
rate(cortex_querier_queries_failed_total[5m])

# Query retries
rate(cortex_query_frontend_retries_total[5m])
```

### Distributor

```promql
# Distributor receive latency
histogram_quantile(0.99, rate(cortex_distributor_sample_delay_seconds_bucket[5m]))

# Push failures
rate(cortex_distributor_receive_grpc_request_failures_total[5m])

# Replication failures
rate(cortex_distributor_replication_factor_failures_total[5m])
```

### Store Gateway

```promql
# Block sync latency
histogram_quantile(0.99, rate(cortex_storegateway_series_fetch_duration_seconds_bucket[5m]))

# Cache hit rate
rate(cortex_storegateway_series_result_series_total[5m])
```

---

## Claude's Investigation Protocol

When Claude is given access to this MCP server, it should follow this workflow:

### Phase 1 — Orient
1. Call `list_metrics` with `{job=~"mimir.*"}` to confirm what metrics are available
2. Check the time range of the load test (ask user if not known)
3. Ask the user what issue they are investigating (e.g. missed evaluations, high latency, errors)
4. Call `run_diagnostic_bundle` with the most relevant subsystem for the reported issue

### Phase 2 — Confirm the symptom
Use the appropriate metrics to confirm the user-reported issue exists in the data.
For example, for missed evaluations:
```promql
increase(cortex_ruler_rule_evaluation_missed_iterations_total[<test_duration>])
rate(cortex_ruler_rule_evaluations_total[5m])
```

### Phase 3 — Trace the bottleneck
Work down the call path for a rule evaluation:

```
Ruler scheduler → picks rule group
    → queries Querier (via query frontend)
        → Querier fetches from Ingester + Store Gateway
    → evaluates result
    → writes back via Distributor → Ingester
```

At each hop, check:
- **Latency** (p99 histograms)
- **Error rates**
- **Queue depths / backpressure**
- **Resource saturation** (if node_exporter metrics available)

### Phase 4 — Correlate timing
Use `query_range` to overlay:
- `cortex_ruler_rule_evaluation_missed_iterations_total` (rate)
- `cortex_query_frontend_queue_length`
- `cortex_ingester_memory_series`
- `cortex_ruler_rule_evaluation_duration_seconds` (p99)

Look for which metric **rises before** missed evaluations increase — that's your bottleneck.

### Phase 5 — Report
Produce a structured markdown report with:
- **Executive Summary**: what is the bottleneck
- **Evidence**: the specific metrics and values that show it
- **Timeline**: when the bottleneck started relative to load increase
- **Scaling recommendation**: what to scale, by how much, and why
- **Follow-up queries**: metrics to watch after scaling

---

## Build & Run

```bash
# Build
go build -o mimir-analyzer ./...

# Run with Claude Code (stdio MCP)
export AMP_ENDPOINT=https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxxxx
export AWS_REGION=us-east-1
export LOADTEST_START=2024-01-15T10:00:00Z
export LOADTEST_END=2024-01-15T12:00:00Z
./mimir-analyzer

# Register with Claude Code
# In your .claude/mcp.json:
{
  "mcpServers": {
    "mimir-analyzer": {
      "command": "/path/to/mimir-analyzer",
      "env": {
        "AMP_ENDPOINT": "https://...",
        "AWS_REGION": "us-east-1",
        "LOADTEST_START": "2024-01-15T10:00:00Z",
        "LOADTEST_END": "2024-01-15T12:00:00Z"
      }
    }
  }
}
```

---

## `--help` Output Requirements

The binary **must** implement a `--help` flag that prints everything Claude needs
to understand and use the tool without any external documentation. This is important
because Claude Code will run `--help` when it first encounters the binary.

The output must cover: required env vars, what each tool does, the load test window
behaviour, and example PromQL patterns for Mimir.

### Expected `--help` output

```
mimir-analyzer — Mimir load test bottleneck analyzer (MCP server)

USAGE
  mimir-analyzer            Start the MCP server (stdio transport, for Claude Code)
  mimir-analyzer --help     Print this help

REQUIRED ENVIRONMENT VARIABLES
  AMP_ENDPOINT        Amazon Managed Prometheus workspace URL
                      e.g. https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxxxx
  AWS_REGION          AWS region of the AMP workspace, e.g. us-east-1
  LOADTEST_START      Start of the load test window, RFC3339
                      e.g. 2024-01-15T10:00:00Z
  LOADTEST_END        End of the load test window, RFC3339
                      e.g. 2024-01-15T12:00:00Z

OPTIONAL ENVIRONMENT VARIABLES
  QUERY_TIMEOUT_SECONDS   HTTP timeout for AMP queries (default: 30)
  MAX_SERIES_RETURNED     Cap on series returned per query (default: 2000)

AWS AUTHENTICATION
  Uses the default AWS credential chain. Credentials must already be present
  in the environment (e.g. via environment variables, ~/.aws/credentials, or
  an instance profile). If credentials expire mid-session, tool calls will
  return a clear error: "AWS credentials expired or invalid"

MCP TOOLS
  query_instant
    Run a PromQL instant query. Time is clamped to [LOADTEST_START, LOADTEST_END].
    Defaults to LOADTEST_END if no time given.
    Input:  expr (required), time (optional, RFC3339)

  query_range
    Run a PromQL range query for trend analysis and correlation.
    start/end are clamped to [LOADTEST_START, LOADTEST_END] and default to the
    full window if omitted. Step defaults to 1m.
    Input:  expr (required), start, end, step (all optional)

  list_metrics
    Discover available metric names, scoped to the load test window.
    Use before querying to verify metric names exist.
    Input:  match (optional label selector), limit (optional, default 200)

  run_diagnostic_bundle
    Run a pre-built set of diagnostic queries for a Mimir subsystem.
    Covers: ruler, ingester, querier, distributor, compactor, store_gateway,
            query_frontend, all
    Runs over the full load test window by default; start/end can narrow it.
    Input:  subsystem (required), start, end (optional)

INVESTIGATION APPROACH
  1. Start with: run_diagnostic_bundle subsystem=ruler
     Ruler is the primary suspect for missed evaluations.
  2. Confirm the symptom:
       increase(cortex_ruler_rule_evaluation_missed_iterations_total[<window>])
  3. Trace the call path:
       ruler → query_frontend → querier → ingester / store_gateway
               ↓ writes back via
       distributor → ingester
     At each hop check p99 latency, error rates, and queue depths.
  4. Use query_range to correlate: which metric rose *before* missed evaluations?
  5. Produce a report: bottleneck component, evidence, scaling recommendation.

KEY MIMIR METRICS FOR MISSED EVALUATIONS
  cortex_ruler_rule_evaluation_missed_iterations_total   — the primary symptom
  cortex_ruler_rule_evaluation_duration_seconds          — ruler evaluation latency
  cortex_ruler_rule_group_duration_seconds               — full group eval latency
  cortex_ruler_queries_failed_total                      — querier errors from ruler
  cortex_query_frontend_queue_length                     — query backpressure
  cortex_querier_query_duration_seconds                  — querier latency
  cortex_ingester_memory_series                          — ingester memory pressure
  cortex_ring_members{name="ruler"}                      — ruler ring health

EXAMPLE QUERIES
  # How many evaluations were missed during the load test?
  increase(cortex_ruler_rule_evaluation_missed_iterations_total[2h])

  # p99 evaluation latency over time
  histogram_quantile(0.99, rate(cortex_ruler_rule_evaluation_duration_seconds_bucket[5m]))

  # Is the query frontend backed up?
  cortex_query_frontend_queue_length

  # Querier error rate
  rate(cortex_querier_queries_failed_total[5m])
```

### Implementation in Go

Use the `flag` package with a custom usage function. Print to stdout (not stderr)
so Claude can capture it cleanly:

```go
// main.go
func main() {
    help := flag.Bool("help", false, "Print usage")
    flag.Parse()

    if *help {
        fmt.Print(helpText) // helpText is the full string above, as a const
        os.Exit(0)
    }

    // validate config, start MCP server...
}
```

Define `helpText` as a package-level `const` in `main.go` — not generated
dynamically — so it is always accurate and complete regardless of env vars being set.

---

## Go Dependencies

```go
// go.mod
module github.com/your-org/mimir-bottleneck-analyzer

go 1.22

require (
    github.com/aws/aws-sdk-go-v2                v1.26.0
    github.com/aws/aws-sdk-go-v2/config         v1.27.0  // handles default cred chain incl. ~/.aws/credentials
    // SigV4 signing for HTTP
    github.com/aws/aws-sdk-go-v2/aws/signer/v4  v4.7.0
    // Prometheus HTTP API client
    github.com/prometheus/client_golang          v1.19.0
    // MCP server — use the official Go SDK
    github.com/mark3labs/mcp-go                 v0.8.0
)
```

---

## Key Implementation Notes

### Load Test Window Clamping

`config/config.go` parses `LOADTEST_START` and `LOADTEST_END` at startup and
stores them as `time.Time`. Every query path calls `clampToWindow` before
issuing the request to AMP — Claude cannot accidentally query outside the window.

```go
// config/config.go
type Config struct {
    AMPEndpoint    string
    AWSRegion      string
    LoadtestStart  time.Time
    LoadtestEnd    time.Time
    QueryTimeout   time.Duration
    MaxSeries      int
}

func Load() (*Config, error) {
    start, err := time.Parse(time.RFC3339, os.Getenv("LOADTEST_START"))
    if err != nil {
        return nil, fmt.Errorf("LOADTEST_START must be RFC3339: %w", err)
    }
    end, err := time.Parse(time.RFC3339, os.Getenv("LOADTEST_END"))
    if err != nil {
        return nil, fmt.Errorf("LOADTEST_END must be RFC3339: %w", err)
    }
    if !end.After(start) {
        return nil, fmt.Errorf("LOADTEST_END must be after LOADTEST_START")
    }
    // ... rest of fields
}

// clampToWindow enforces that start/end never exceed the configured window.
// Called by every query handler before hitting AMP.
func (c *Config) ClampToWindow(start, end time.Time) (time.Time, time.Time) {
    if start.IsZero() || start.Before(c.LoadtestStart) {
        start = c.LoadtestStart
    }
    if end.IsZero() || end.After(c.LoadtestEnd) {
        end = c.LoadtestEnd
    }
    return start, end
}
```

The window is also injected into every tool response under a `"window"` field so
Claude always sees the actual time range that was queried:

```json
{
  "window": { "start": "2024-01-15T10:00:00Z", "end": "2024-01-15T12:00:00Z" },
  "status": "ok",
  "query": "...",
  "data": [...]
}
```

### SigV4 Auth for AMP
AMP requires AWS SigV4 signing on every request. The `amp/client.go` wraps the
HTTP client with a transport that signs requests. Credentials come from
`config.LoadDefaultConfig` via the standard AWS SDK default credential chain.

```go
// amp/client.go
type sigV4RoundTripper struct {
    signer  *v4.Signer
    creds   aws.CredentialsProvider  // sourced from config.LoadDefaultConfig
    region  string
    next    http.RoundTripper
}

func (t *sigV4RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    // Buffer body for signing (required by SigV4)
    var bodyBytes []byte
    if req.Body != nil {
        bodyBytes, _ = io.ReadAll(req.Body)
        req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
    }
    hash := sha256Hex(bodyBytes)

    creds, err := t.creds.Retrieve(req.Context())
    if err != nil {
        return nil, fmt.Errorf("retrieving AWS credentials: %w", err)
    }

    err = t.signer.SignHTTP(req.Context(), creds, req, hash, "aps", t.region, time.Now())
    if err != nil {
        return nil, fmt.Errorf("signing request: %w", err)
    }

    resp, err := t.next.RoundTrip(req)
    if err != nil {
        return nil, err
    }

    // Surface credential expiry as a clear, actionable error
    if resp.StatusCode == 401 || resp.StatusCode == 403 {
        return nil, fmt.Errorf(
            "AWS credentials expired or invalid (HTTP %d) — ensure valid credentials are present in your environment",
            resp.StatusCode,
        )
    }
    return resp, nil
}
```

### MCP Transport
Use **stdio** transport (not HTTP) — this is what Claude Code and the claude.ai
MCP integration expect for local servers.

```go
// main.go
server := mcp.NewServer("mimir-analyzer", "1.0.0")
server.AddTool(queryInstantTool, handleQueryInstant)
server.AddTool(queryRangeTool, handleQueryRange)
server.AddTool(listMetricsTool, handleListMetrics)
server.AddTool(diagnosticBundleTool, handleDiagnosticBundle)
server.ServeStdio() // blocks, reads JSON-RPC from stdin
```

### Response Formatting
Keep tool responses **concise but structured**. Claude needs to reason over them,
not render them. Return JSON with:
- `status`: "ok" | "error" | "empty"
- `query`: the expr that was run (for traceability)
- `summary`: 1–2 line human interpretation when possible
- `data`: the actual results

Cap returned series at `MAX_SERIES_RETURNED`. If truncated, say so.

---

## What Claude Should NOT Do

- Do not issue queries with very small step values over long ranges (e.g., `step=1s` over `1h`) — this produces millions of points and will time out
- Do not assume a bottleneck before seeing evidence — follow the call path systematically
- Do not recommend scaling a component without showing the saturation metric that justifies it
- Do not stop at the first anomaly — correlate across multiple subsystems before concluding

---

## Expected Output: Bottleneck Report Structure

```markdown
# Mimir Load Test Bottleneck Analysis
**Test parameters:** [user-provided description], [start] – [end]
**Primary symptom:** [describe the issue reported by the user]

## Verdict
[One paragraph: what is the bottleneck and why]

## Evidence

### Primary Bottleneck: [Component]
| Metric | Value at Peak | Threshold / Normal |
|--------|--------------|-------------------|
| ...    | ...          | ...               |

### Contributing Factors
...

## Timeline
[When did the bottleneck appear relative to load ramp-up]

## Scaling Recommendation
| Component | Current | Recommended | Rationale |
|-----------|---------|-------------|-----------|
| ...       | ...     | ...         | ...       |

## Metrics to Watch After Scaling
...
```