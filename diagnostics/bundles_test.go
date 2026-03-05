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

func TestRunBundle_FallsBackToAlternativeWhenPrimaryEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		// Primary returns empty, alternative returns data
		if query == "primary_expr" {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		} else if query == "alt_expr" {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"test"},"values":[[1705312800,"1"]]}]}}`)
		} else {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	client := amp.NewTestClient(server.URL, cfg)

	// Register a test bundle with alternatives
	origBundles := subsystemBundles
	subsystemBundles = map[string]func() Bundle{
		"test_alt": func() Bundle {
			return Bundle{
				Name: "test_alt",
				Queries: []DiagnosticQuery{
					{
						Name:         "test_query",
						Expr:         "primary_expr",
						Description:  "test",
						Alternatives: []string{"alt_expr"},
					},
				},
			}
		},
	}
	defer func() { subsystemBundles = origBundles }()

	result, err := RunBundle(context.Background(), client, cfg, "test_alt", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(result.Results))
	}
	entry := result.Results[0]
	if entry.Query != "alt_expr" {
		t.Errorf("query = %q, want %q (should reflect the alternative that succeeded)", entry.Query, "alt_expr")
	}
	if entry.Result == nil || entry.Result.Status != "ok" {
		t.Errorf("expected result with status ok, got %v", entry.Result)
	}
}

func TestRunBundle_PrimarySucceeds_NoAlternativesTried(t *testing.T) {
	queriesExecuted := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queriesExecuted++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"test"},"values":[[1705312800,"1"]]}]}}`)
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	client := amp.NewTestClient(server.URL, cfg)

	origBundles := subsystemBundles
	subsystemBundles = map[string]func() Bundle{
		"test_noalt": func() Bundle {
			return Bundle{
				Name: "test_noalt",
				Queries: []DiagnosticQuery{
					{
						Name:         "q1",
						Expr:         "primary1",
						Description:  "test",
						Alternatives: []string{"alt1"},
					},
					{
						Name:        "q2",
						Expr:        "primary2",
						Description: "test",
					},
				},
			}
		},
	}
	defer func() { subsystemBundles = origBundles }()

	_, err := RunBundle(context.Background(), client, cfg, "test_noalt", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both primaries succeed, so only 2 queries should be executed
	if queriesExecuted != 2 {
		t.Errorf("queriesExecuted = %d, want 2 (alternatives should not be tried)", queriesExecuted)
	}
}

func TestRunBundle_RecordsActualExprUsed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		if query == "alt2" {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"test"},"values":[[1705312800,"1"]]}]}}`)
		} else {
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	client := amp.NewTestClient(server.URL, cfg)

	origBundles := subsystemBundles
	subsystemBundles = map[string]func() Bundle{
		"test_record": func() Bundle {
			return Bundle{
				Name: "test_record",
				Queries: []DiagnosticQuery{
					{
						Name:         "q",
						Expr:         "primary",
						Description:  "test",
						Alternatives: []string{"alt1", "alt2"},
					},
				},
			}
		},
	}
	defer func() { subsystemBundles = origBundles }()

	result, err := RunBundle(context.Background(), client, cfg, "test_record", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry := result.Results[0]
	if entry.Query != "alt2" {
		t.Errorf("query = %q, want %q (should record the winning alternative)", entry.Query, "alt2")
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
