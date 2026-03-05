package amp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mimir-analyzer/config"
)

func TestListMetrics_DefaultLimit(t *testing.T) {
	// Generate more than 200 metrics
	metrics := make([]string, 250)
	for i := range metrics {
		metrics[i] = fmt.Sprintf(`"metric_%d"`, i)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"success","data":[%s]}`, joinStrings(metrics))
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	c := newTestClient(server.URL)
	c.config = cfg

	result, err := c.ListMetrics(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) > 200 {
		t.Errorf("got %d metrics, want at most 200 (default limit)", len(result.Data))
	}
}

func TestListMetrics_WithMatch(t *testing.T) {
	var capturedMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMatch = r.URL.Query().Get("match[]")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":["cortex_ruler_rule_evaluations_total"]}`)
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	c := newTestClient(server.URL)
	c.config = cfg

	_, err := c.ListMetrics(context.Background(), `{job=~"mimir.*"}`, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMatch != `{job=~"mimir.*"}` {
		t.Errorf("match[] = %q, want %q", capturedMatch, `{job=~"mimir.*"}`)
	}
}

func TestListMetrics_RespectsLimit(t *testing.T) {
	metrics := make([]string, 50)
	for i := range metrics {
		metrics[i] = fmt.Sprintf(`"metric_%d"`, i)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"success","data":[%s]}`, joinStrings(metrics))
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	c := newTestClient(server.URL)
	c.config = cfg

	result, err := c.ListMetrics(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Data) > 10 {
		t.Errorf("got %d metrics, want at most 10", len(result.Data))
	}
}

func TestListMetrics_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"error","errorType":"bad_data","error":"invalid selector"}`)
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	c := newTestClient(server.URL)
	c.config = cfg

	_, err := c.ListMetrics(context.Background(), "bad{", 0)
	if err == nil {
		t.Fatal("expected error for error status response")
	}
	if !strings.Contains(err.Error(), "invalid selector") {
		t.Errorf("error should contain server message, got: %v", err)
	}
}

func TestListMetrics_UnreachableEndpoint(t *testing.T) {
	c := newTestClient("http://127.0.0.1:1")

	_, err := c.ListMetrics(context.Background(), "", 0)
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

func TestListMetrics_EmptySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":[]}`)
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	c := newTestClient(server.URL)
	c.config = cfg

	result, err := c.ListMetrics(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("status = %q, want %q", result.Status, "ok")
	}
	if len(result.Data) != 0 {
		t.Errorf("data length = %d, want 0", len(result.Data))
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
