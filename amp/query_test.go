package amp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mimir-analyzer/config"
)

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

func newTestClient(serverURL string) *Client {
	return &Client{
		endpoint:   serverURL,
		httpClient: &http.Client{},
	}
}

func TestQueryInstant_SerializesParams(t *testing.T) {
	var capturedQuery, capturedTime string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		capturedTime = r.URL.Query().Get("time")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up"},"value":[1705312800,"1"]}]}}`)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	queryTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	result, err := c.QueryInstant(context.Background(), "up", queryTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery != "up" {
		t.Errorf("query = %q, want %q", capturedQuery, "up")
	}
	if capturedTime == "" {
		t.Error("time param was not sent")
	}
	if result.Status != "ok" {
		t.Errorf("status = %q, want %q", result.Status, "ok")
	}
}

func TestQueryInstant_EmptyResult(t *testing.T) {
	server := newFakeAMPServer(t, map[string]string{
		"nonexistent": `{"status":"success","data":{"resultType":"vector","result":[]}}`,
	})
	defer server.Close()

	c := newTestClient(server.URL)
	result, err := c.QueryInstant(context.Background(), "nonexistent", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "empty" {
		t.Errorf("status = %q, want %q", result.Status, "empty")
	}
}

func TestQueryRange_SerializesParams(t *testing.T) {
	var capturedQuery, capturedStart, capturedEnd, capturedStep string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("query")
		capturedStart = r.URL.Query().Get("start")
		capturedEnd = r.URL.Query().Get("end")
		capturedStep = r.URL.Query().Get("step")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"up"},"values":[[1705312800,"1"]]}]}}`)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	result, err := c.QueryRange(context.Background(), "up", start, end, "1m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery != "up" {
		t.Errorf("query = %q, want %q", capturedQuery, "up")
	}
	if capturedStart == "" {
		t.Error("start param was not sent")
	}
	if capturedEnd == "" {
		t.Error("end param was not sent")
	}
	if capturedStep != "1m" {
		t.Errorf("step = %q, want %q", capturedStep, "1m")
	}
	if result.Status != "ok" {
		t.Errorf("status = %q, want %q", result.Status, "ok")
	}
}

func TestQueryRange_EmptyResult(t *testing.T) {
	server := newFakeAMPServer(t, map[string]string{
		"nonexistent": `{"status":"success","data":{"resultType":"matrix","result":[]}}`,
	})
	defer server.Close()

	c := newTestClient(server.URL)
	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	result, err := c.QueryRange(context.Background(), "nonexistent", start, end, "1m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "empty" {
		t.Errorf("status = %q, want %q", result.Status, "empty")
	}
}

func TestQueryRange_ClampsToWindow(t *testing.T) {
	var capturedStart, capturedEnd string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("start")
		capturedEnd = r.URL.Query().Get("end")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	c := newTestClient(server.URL)
	c.config = cfg

	// Request range that exceeds the window
	start := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC) // before window
	end := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)   // after window

	_, err := c.QueryRange(context.Background(), "up", start, end, "1m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the captured params show clamped values
	expectedStart := cfg.LoadtestStart.Format(time.RFC3339)
	expectedEnd := cfg.LoadtestEnd.Format(time.RFC3339)
	if capturedStart != expectedStart {
		t.Errorf("start = %q, want clamped to %q", capturedStart, expectedStart)
	}
	if capturedEnd != expectedEnd {
		t.Errorf("end = %q, want clamped to %q", capturedEnd, expectedEnd)
	}
}

func TestQueryInstant_UnreachableEndpoint(t *testing.T) {
	// Use a port that is guaranteed to refuse connections
	c := newTestClient("http://127.0.0.1:1")

	_, err := c.QueryInstant(context.Background(), "up", time.Now())
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatalf("error should be *AMPError, got %T: %v", err, err)
	}
	if ampErr.Category != CategoryConnectivity {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryConnectivity)
	}
}

func TestQueryRange_UnreachableEndpoint(t *testing.T) {
	c := newTestClient("http://127.0.0.1:1")
	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	_, err := c.QueryRange(context.Background(), "up", start, end, "1m")
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatalf("error should be *AMPError, got %T: %v", err, err)
	}
	if ampErr.Category != CategoryConnectivity {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryConnectivity)
	}
}
