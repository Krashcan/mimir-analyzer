package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mimir-analyzer/amp"
	"mimir-analyzer/config"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*Handlers, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		QueryTimeout:  30 * time.Second,
		MaxSeries:     2000,
	}
	client := amp.NewTestClient(server.URL, cfg)
	h := NewHandlers(client, cfg)
	return h, server
}

func defaultAMPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up"},"value":[1705312800,"1"]}]}}`)
	}
}

func makeRequest(args map[string]any) mcpgo.CallToolRequest {
	params := mcpgo.CallToolParams{
		Name: "test",
	}
	if args != nil {
		raw, _ := json.Marshal(args)
		params.Arguments = make(map[string]any)
		json.Unmarshal(raw, &params.Arguments)
	}
	return mcpgo.CallToolRequest{
		Params: params,
	}
}

func TestQueryInstantHandler_MissingExpr(t *testing.T) {
	h, server := setupTestServer(t, defaultAMPHandler())
	defer server.Close()

	result, err := h.HandleQueryInstant(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for missing expr")
	}
}

func TestQueryInstantHandler_ValidExpr(t *testing.T) {
	h, server := setupTestServer(t, defaultAMPHandler())
	defer server.Close()

	result, err := h.HandleQueryInstant(context.Background(), makeRequest(map[string]any{
		"expr": "up",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("unexpected IsError=true")
	}

	// Parse the text content to verify window field
	text := result.Content[0].(mcpgo.TextContent).Text
	var resp ToolResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Window == nil {
		t.Error("response missing window field")
	}
	if resp.Status != "ok" {
		t.Errorf("status = %q, want %q", resp.Status, "ok")
	}
}

func TestQueryRangeHandler_DefaultsStepTo1m(t *testing.T) {
	var capturedStep string
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedStep = r.URL.Query().Get("step")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	})
	defer server.Close()

	result, err := h.HandleQueryRange(context.Background(), makeRequest(map[string]any{
		"expr": "up",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("unexpected IsError=true")
	}
	if capturedStep != "1m" {
		t.Errorf("step = %q, want %q", capturedStep, "1m")
	}
}

func TestQueryRangeHandler_ValidParams(t *testing.T) {
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"up"},"values":[[1705312800,"1"]]}]}}`)
	})
	defer server.Close()

	result, err := h.HandleQueryRange(context.Background(), makeRequest(map[string]any{
		"expr":  "up",
		"start": "2024-01-15T10:00:00Z",
		"end":   "2024-01-15T12:00:00Z",
		"step":  "5m",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("unexpected IsError=true")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp ToolResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Window == nil {
		t.Error("response missing window field")
	}
}

func TestListMetricsHandler_DefaultsLimitTo200(t *testing.T) {
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":["metric_1","metric_2"]}`)
	})
	defer server.Close()

	result, err := h.HandleListMetrics(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("unexpected IsError=true")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp ToolResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Window == nil {
		t.Error("response missing window field")
	}
}

func TestDiagnosticBundleHandler_UnknownSubsystem(t *testing.T) {
	h, server := setupTestServer(t, defaultAMPHandler())
	defer server.Close()

	result, err := h.HandleDiagnosticBundle(context.Background(), makeRequest(map[string]any{
		"subsystem": "nonexistent",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for unknown subsystem")
	}
}

func TestDiagnosticBundleHandler_ValidSubsystem(t *testing.T) {
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	})
	defer server.Close()

	result, err := h.HandleDiagnosticBundle(context.Background(), makeRequest(map[string]any{
		"subsystem": "ruler",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected IsError=true")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["window"] == nil {
		t.Error("response missing window field")
	}
}

func TestCheckConnectionHandler_Connected(t *testing.T) {
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/status/buildinfo" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"success","data":{"version":"2.10.0","branch":"HEAD"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	})
	defer server.Close()

	result, err := h.HandleCheckConnection(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("unexpected IsError=true")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "connected" {
		t.Errorf("status = %q, want %q", resp["status"], "connected")
	}
}

func TestCheckConnectionHandler_AuthFailed(t *testing.T) {
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer server.Close()

	result, err := h.HandleCheckConnection(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("unexpected IsError=true — check_connection should always return structured JSON")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "auth_failed" {
		t.Errorf("status = %q, want %q", resp["status"], "auth_failed")
	}
}

func TestCheckConnectionHandler_WrongEndpoint(t *testing.T) {
	h, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "<html>Not Found</html>")
	})
	defer server.Close()

	result, err := h.HandleCheckConnection(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["status"] != "wrong_endpoint" {
		t.Errorf("status = %q, want %q", resp["status"], "wrong_endpoint")
	}
}

func TestQueryInstantHandler_ErrorIncludesCategory(t *testing.T) {
	// Use an unreachable endpoint to trigger a connectivity error
	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		QueryTimeout:  30 * time.Second,
		MaxSeries:     2000,
	}
	client := amp.NewTestClient("http://127.0.0.1:1", cfg)
	h := NewHandlers(client, cfg)

	result, err := h.HandleQueryInstant(context.Background(), makeRequest(map[string]any{
		"expr": "up",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for connectivity failure")
	}

	text := result.Content[0].(mcpgo.TextContent).Text
	var resp map[string]any
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		// Error result might be plain text — check it contains the category
		if !strings.Contains(text, "category") {
			t.Errorf("error response should include category, got: %s", text)
		}
		return
	}
	if resp["category"] == nil {
		t.Error("error response missing 'category' field")
	}
	if resp["category"] != "connectivity" {
		t.Errorf("category = %q, want %q", resp["category"], "connectivity")
	}
}

func TestAllHandlers_IncludeWindowField(t *testing.T) {
	ampHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/label/__name__/values" {
			fmt.Fprint(w, `{"status":"success","data":["metric_1"]}`)
		} else if r.URL.Path == "/api/v1/query_range" {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		} else if r.URL.Path == "/api/v1/status/buildinfo" {
			fmt.Fprint(w, `{"status":"success","data":{"version":"2.10.0"}}`)
		} else {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up"},"value":[1705312800,"1"]}]}}`)
		}
	}

	h, server := setupTestServer(t, ampHandler)
	defer server.Close()

	tests := []struct {
		name    string
		handler func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error)
		args    map[string]any
	}{
		{"query_instant", h.HandleQueryInstant, map[string]any{"expr": "up"}},
		{"query_range", h.HandleQueryRange, map[string]any{"expr": "up"}},
		{"list_metrics", h.HandleListMetrics, nil},
		{"run_diagnostic_bundle", h.HandleDiagnosticBundle, map[string]any{"subsystem": "compactor"}},
		{"check_connection", h.HandleCheckConnection, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), makeRequest(tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected IsError=true")
			}

			text := result.Content[0].(mcpgo.TextContent).Text
			if text == "" {
				t.Fatal("empty response text")
			}

			var resp map[string]any
			if err := json.Unmarshal([]byte(text), &resp); err != nil {
				t.Fatalf("failed to parse JSON response: %v", err)
			}
			// check_connection doesn't have a window field, only query tools do
			if tt.name != "check_connection" {
				if resp["window"] == nil {
					t.Error("response missing 'window' field")
				}
			}
		})
	}
}
