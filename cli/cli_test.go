package cli

import (
	"bytes"
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
)

func setupCLITest(t *testing.T, handler http.HandlerFunc) (*amp.Client, *config.Config, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		QueryTimeout:  30 * time.Second,
		MaxSeries:     2000,
	}
	client := amp.NewTestClient(server.URL, cfg)
	return client, cfg, server
}

func defaultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/query":
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up"},"value":[1705312800,"1"]}]}}`)
		case "/api/v1/query_range":
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"up"},"values":[[1705312800,"1"]]}]}}`)
		case "/api/v1/label/__name__/values":
			fmt.Fprint(w, `{"status":"success","data":["metric_1","metric_2"]}`)
		case "/api/v1/labels":
			fmt.Fprint(w, `{"status":"success","data":["__name__","job","instance"]}`)
		default:
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		}
	}
}

func TestRun_NoArgs_ShowsHelp(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), nil, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "USAGE") {
		t.Errorf("expected help output containing USAGE, got: %s", output)
	}
	if !strings.Contains(output, "query") {
		t.Errorf("expected help output listing commands, got: %s", output)
	}
}

func TestRun_Query_OutputsJSON(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"query", "up"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestRun_Query_MissingExpr(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"query"}, client, cfg, &buf)
	if err == nil {
		t.Fatal("expected error for missing expression")
	}
}

func TestRun_CheckConnection(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"check-connection"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if resp["status"] != "connected" {
		t.Errorf("status = %q, want %q", resp["status"], "connected")
	}
}

func TestRun_Diagnose(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"diagnose", "compactor"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if resp["subsystem"] != "compactor" {
		t.Errorf("subsystem = %q, want %q", resp["subsystem"], "compactor")
	}
}

func TestRun_ListMetrics(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"list-metrics"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestRun_QueryRange_WithStep(t *testing.T) {
	var capturedStep string
	client, cfg, server := setupCLITest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/query_range" {
			capturedStep = r.URL.Query().Get("step")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	})
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"query-range", "up", "--step", "5m"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedStep != "5m" {
		t.Errorf("step = %q, want %q", capturedStep, "5m")
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	client, cfg, server := setupCLITest(t, defaultHandler())
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"unknown"}, client, cfg, &buf)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func TestRun_Diagnose_WithStartEndFlags(t *testing.T) {
	var capturedStart, capturedEnd string
	client, cfg, server := setupCLITest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/query_range" {
			capturedStart = r.URL.Query().Get("start")
			capturedEnd = r.URL.Query().Get("end")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	})
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{
		"diagnose", "compactor",
		"--start", "2024-01-15T10:30:00Z",
		"--end", "2024-01-15T11:30:00Z",
	}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedStart != "2024-01-15T10:30:00Z" {
		t.Errorf("start = %q, want %q", capturedStart, "2024-01-15T10:30:00Z")
	}
	if capturedEnd != "2024-01-15T11:30:00Z" {
		t.Errorf("end = %q, want %q", capturedEnd, "2024-01-15T11:30:00Z")
	}
}

func TestRun_Diagnose_DefaultOmitsRawData(t *testing.T) {
	client, cfg, server := setupCLITest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"test"},"values":[[1705312800,"1"],[1705312860,"2"]]}]}}`)
	})
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"diagnose", "compactor"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	results := resp["results"].([]any)
	for _, r := range results {
		entry := r.(map[string]any)
		if res, ok := entry["result"].(map[string]any); ok {
			if res["data"] != nil {
				t.Errorf("entry %q should not have raw data in default (non-verbose) mode", entry["name"])
			}
		}
	}
}

func TestRun_Diagnose_WithVerboseFlag(t *testing.T) {
	client, cfg, server := setupCLITest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"test"},"values":[[1705312800,"1"],[1705312860,"2"]]}]}}`)
	})
	defer server.Close()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"diagnose", "compactor", "--verbose"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	results := resp["results"].([]any)
	hasData := false
	for _, r := range results {
		entry := r.(map[string]any)
		if res, ok := entry["result"].(map[string]any); ok {
			if res["data"] != nil {
				hasData = true
			}
		}
	}
	if !hasData {
		t.Error("verbose mode should include raw data in at least one result")
	}
}

func TestRun_Diagnose_ErrorIncludesCategory(t *testing.T) {
	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		QueryTimeout:  30 * time.Second,
		MaxSeries:     2000,
	}
	client := amp.NewTestClient("http://127.0.0.1:1", cfg)

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"query", "up"}, client, cfg, &buf)
	if err != nil {
		t.Fatalf("expected structured JSON output, got error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if resp["status"] != "error" {
		t.Errorf("status = %q, want %q", resp["status"], "error")
	}
	if resp["category"] == nil || resp["category"] == "" {
		t.Error("error response missing 'category' field")
	}
}
