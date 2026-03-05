package mcp

import (
	"encoding/json"
)

// ToolResponse is the JSON structure returned by all MCP tool handlers.
type ToolResponse struct {
	Status   string          `json:"status"`
	Query    string          `json:"query,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Summary  string          `json:"summary,omitempty"`
	Window   *WindowInfo     `json:"window"`
	Error    string          `json:"error,omitempty"`
	Category string          `json:"category,omitempty"`
}

type WindowInfo struct {
	Start string `json:"start"`
	End   string `json:"end"`
}
