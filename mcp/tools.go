package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"mimir-analyzer/amp"
	"mimir-analyzer/config"
	"mimir-analyzer/diagnostics"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type Handlers struct {
	client *amp.Client
	config *config.Config
}

func NewHandlers(client *amp.Client, cfg *config.Config) *Handlers {
	return &Handlers{client: client, config: cfg}
}

func errorResult(msg string) *mcpgo.CallToolResult {
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: msg},
		},
		IsError: true,
	}
}

// categorizedErrorResult returns a JSON error response with a category field
// so the LLM can reason about what type of failure occurred.
func categorizedErrorResult(category, msg string, window *WindowInfo) *mcpgo.CallToolResult {
	resp := ToolResponse{
		Status:   "error",
		Error:    msg,
		Category: category,
		Window:   window,
	}
	data, _ := json.MarshalIndent(resp, "", "  ")
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: string(data)},
		},
		IsError: true,
	}
}

// ampErrorResult extracts the category from an *AMPError (if present) and returns
// a categorized error result. Falls back to a plain error result if not an AMPError.
func (h *Handlers) ampErrorResult(prefix string, err error) *mcpgo.CallToolResult {
	var ampErr *amp.AMPError
	if errors.As(err, &ampErr) {
		return categorizedErrorResult(string(ampErr.Category), prefix+ampErr.Message, h.windowInfo())
	}
	return categorizedErrorResult("", prefix+err.Error(), h.windowInfo())
}

func jsonResult(v any) *mcpgo.CallToolResult {
	data, _ := json.MarshalIndent(v, "", "  ")
	return &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: string(data)},
		},
	}
}

func (h *Handlers) windowInfo() *WindowInfo {
	return &WindowInfo{
		Start: h.config.LoadtestStart.Format(time.RFC3339),
		End:   h.config.LoadtestEnd.Format(time.RFC3339),
	}
}

func (h *Handlers) HandleQueryInstant(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	expr := req.GetString("expr", "")
	if expr == "" {
		return errorResult("missing required argument: expr"), nil
	}

	timeStr := req.GetString("time", "")
	var queryTime time.Time
	if timeStr != "" {
		var err error
		queryTime, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return errorResult("invalid time format, must be RFC3339: " + err.Error()), nil
		}
	} else {
		queryTime = h.config.LoadtestEnd
	}

	result, err := h.client.QueryInstant(ctx, expr, queryTime)
	if err != nil {
		return h.ampErrorResult("query failed: ", err), nil
	}

	resp := ToolResponse{
		Status: result.Status,
		Query:  result.Query,
		Data:   result.Data,
		Window: h.windowInfo(),
	}
	return jsonResult(resp), nil
}

func (h *Handlers) HandleQueryRange(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	expr := req.GetString("expr", "")
	if expr == "" {
		return errorResult("missing required argument: expr"), nil
	}

	startStr := req.GetString("start", "")
	endStr := req.GetString("end", "")
	step := req.GetString("step", "1m")

	var start, end time.Time
	if startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return errorResult("invalid start format, must be RFC3339: " + err.Error()), nil
		}
	}
	if endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return errorResult("invalid end format, must be RFC3339: " + err.Error()), nil
		}
	}

	result, err := h.client.QueryRange(ctx, expr, start, end, step)
	if err != nil {
		return h.ampErrorResult("query failed: ", err), nil
	}

	resp := ToolResponse{
		Status: result.Status,
		Query:  result.Query,
		Data:   result.Data,
		Window: h.windowInfo(),
	}
	return jsonResult(resp), nil
}

func (h *Handlers) HandleListMetrics(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	match := req.GetString("match", "")
	limit := req.GetInt("limit", 200)

	result, err := h.client.ListMetrics(ctx, match, limit)
	if err != nil {
		return h.ampErrorResult("list_metrics failed: ", err), nil
	}

	resp := struct {
		Status    string      `json:"status"`
		Data      []string    `json:"data"`
		Truncated bool        `json:"truncated,omitempty"`
		Window    *WindowInfo `json:"window"`
	}{
		Status:    result.Status,
		Data:      result.Data,
		Truncated: result.Truncated,
		Window:    h.windowInfo(),
	}
	return jsonResult(resp), nil
}

func (h *Handlers) HandleDiagnosticBundle(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	subsystem := req.GetString("subsystem", "")
	if subsystem == "" {
		return errorResult("missing required argument: subsystem"), nil
	}

	startStr := req.GetString("start", "")
	endStr := req.GetString("end", "")

	var start, end time.Time
	if startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return errorResult("invalid start format, must be RFC3339: " + err.Error()), nil
		}
	}
	if endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return errorResult("invalid end format, must be RFC3339: " + err.Error()), nil
		}
	}

	result, err := diagnostics.RunBundle(ctx, h.client, h.config, subsystem, start, end)
	if err != nil {
		return h.ampErrorResult("", err), nil
	}

	return jsonResult(result), nil
}

func (h *Handlers) HandleCheckConnection(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	status := h.client.CheckConnection(ctx)
	return jsonResult(status), nil
}
