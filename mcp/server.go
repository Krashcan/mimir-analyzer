package mcp

import (
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func NewServer(handlers *Handlers) *server.MCPServer {
	s := server.NewMCPServer("mimir-analyzer", "1.0.0")

	queryInstantTool := mcpgo.NewTool("query_instant",
		mcpgo.WithDescription("Run a PromQL instant query against the AMP workspace. Time is automatically clamped to the configured load test window. Defaults to LOADTEST_END if no time given."),
		mcpgo.WithString("expr",
			mcpgo.Required(),
			mcpgo.Description("PromQL expression"),
		),
		mcpgo.WithString("time",
			mcpgo.Description("RFC3339 timestamp within the load test window. Defaults to LOADTEST_END."),
		),
	)

	queryRangeTool := mcpgo.NewTool("query_range",
		mcpgo.WithDescription("Run a PromQL range query. start/end are clamped to the configured load test window. Defaults to the full window if not specified. Use step wisely — smaller steps = more data = slower."),
		mcpgo.WithString("expr",
			mcpgo.Required(),
			mcpgo.Description("PromQL expression"),
		),
		mcpgo.WithString("start",
			mcpgo.Description("RFC3339. Clamped to LOADTEST_START if earlier. Defaults to LOADTEST_START."),
		),
		mcpgo.WithString("end",
			mcpgo.Description("RFC3339. Clamped to LOADTEST_END if later. Defaults to LOADTEST_END."),
		),
		mcpgo.WithString("step",
			mcpgo.Description("Duration like 15s, 1m, 5m. Defaults to 1m."),
		),
	)

	listMetricsTool := mcpgo.NewTool("list_metrics",
		mcpgo.WithDescription("List metric names matching an optional label selector, scoped to the load test window."),
		mcpgo.WithString("match",
			mcpgo.Description("Optional label selector, e.g. {job=~'mimir.*'}"),
		),
		mcpgo.WithNumber("limit",
			mcpgo.Description("Max results. Default 200."),
		),
	)

	diagnosticBundleTool := mcpgo.NewTool("run_diagnostic_bundle",
		mcpgo.WithDescription("Run pre-built diagnostic queries for a Mimir subsystem over the load test window. start/end can narrow the window but cannot exceed it."),
		mcpgo.WithString("subsystem",
			mcpgo.Required(),
			mcpgo.Description("Mimir subsystem to diagnose"),
			mcpgo.Enum("ruler", "ingester", "querier", "distributor", "compactor", "store_gateway", "query_frontend", "all"),
		),
		mcpgo.WithString("start",
			mcpgo.Description("Optional narrowing start within the load test window."),
		),
		mcpgo.WithString("end",
			mcpgo.Description("Optional narrowing end within the load test window."),
		),
	)

	checkConnectionTool := mcpgo.NewTool("check_connection",
		mcpgo.WithDescription("Verify connectivity to the AMP workspace. Call this before running queries to confirm the endpoint is reachable and credentials are valid. Returns status: connected, auth_failed, unreachable, wrong_endpoint, or error."),
	)

	s.AddTool(queryInstantTool, handlers.HandleQueryInstant)
	s.AddTool(queryRangeTool, handlers.HandleQueryRange)
	s.AddTool(listMetricsTool, handlers.HandleListMetrics)
	s.AddTool(diagnosticBundleTool, handlers.HandleDiagnosticBundle)
	s.AddTool(checkConnectionTool, handlers.HandleCheckConnection)

	return s
}
