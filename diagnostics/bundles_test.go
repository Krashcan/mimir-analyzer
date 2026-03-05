package diagnostics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mimir-analyzer/amp"
	"mimir-analyzer/config"
)

func TestGetBundle_UnknownSubsystem(t *testing.T) {
	_, err := GetBundle("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown subsystem")
	}
}

func TestGetBundle_AllSubsystem(t *testing.T) {
	bundles, err := GetBundle("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bundles) != 6 {
		t.Errorf("got %d bundles, want 6 (all subsystems)", len(bundles))
	}

	// Collect all query count
	totalQueries := 0
	for _, b := range bundles {
		totalQueries += len(b.Queries)
	}
	if totalQueries == 0 {
		t.Error("all bundles combined returned 0 queries")
	}
}

func TestRunBundle_NarrowsTimeRange(t *testing.T) {
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
	client := amp.NewTestClient(server.URL, cfg)

	// Narrow within window
	start := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	result, err := RunBundle(context.Background(), client, cfg, "compactor", start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStart := start.Format(time.RFC3339)
	expectedEnd := end.Format(time.RFC3339)
	if capturedStart != expectedStart {
		t.Errorf("start = %q, want %q", capturedStart, expectedStart)
	}
	if capturedEnd != expectedEnd {
		t.Errorf("end = %q, want %q", capturedEnd, expectedEnd)
	}
	if result.Window.Start != expectedStart {
		t.Errorf("window start = %q, want %q", result.Window.Start, expectedStart)
	}
}

func TestRunBundle_ClampsNarrowedRange(t *testing.T) {
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
	client := amp.NewTestClient(server.URL, cfg)

	// Narrowed range exceeds window on both sides
	start := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC)

	_, err := RunBundle(context.Background(), client, cfg, "compactor", start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStart := cfg.LoadtestStart.Format(time.RFC3339)
	expectedEnd := cfg.LoadtestEnd.Format(time.RFC3339)
	if capturedStart != expectedStart {
		t.Errorf("start = %q, want clamped to %q", capturedStart, expectedStart)
	}
	if capturedEnd != expectedEnd {
		t.Errorf("end = %q, want clamped to %q", capturedEnd, expectedEnd)
	}
}
