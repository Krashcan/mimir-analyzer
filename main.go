package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/mark3labs/mcp-go/server"

	"mimir-analyzer/amp"
	"mimir-analyzer/config"
	mcpserver "mimir-analyzer/mcp"
)

const helpText = `mimir-analyzer — Mimir load test bottleneck analyzer (MCP server)

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
  check_connection
    Verify connectivity to the AMP workspace before running queries.
    Returns status: connected, auth_failed, unreachable, wrong_endpoint, or error.
    Call this first to confirm the endpoint is reachable and credentials are valid.
    Input:  (none)

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
`

func main() {
	help := flag.Bool("help", false, "Print usage")
	flag.Parse()

	if *help {
		fmt.Print(helpText)
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	client := amp.NewClientWithConfig(cfg, awsCfg.Credentials)
	handlers := mcpserver.NewHandlers(client, cfg)
	s := mcpserver.NewServer(handlers)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
