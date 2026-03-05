package diagnostics

import (
	"strings"
	"testing"
)

func TestRulerBundle_ReturnsExpectedQueries(t *testing.T) {
	b := RulerBundle()
	if b.Name != "ruler" {
		t.Errorf("name = %q, want %q", b.Name, "ruler")
	}
	if len(b.Queries) != 8 {
		t.Errorf("got %d queries, want 8", len(b.Queries))
	}

	// Verify key queries are present
	names := map[string]bool{}
	for _, q := range b.Queries {
		names[q.Name] = true
		if q.Expr == "" {
			t.Errorf("query %q has empty expr", q.Name)
		}
	}
	if !names["missed_evaluations"] {
		t.Error("missing 'missed_evaluations' query")
	}
	if !names["rule_evaluation_latency_p99"] {
		t.Error("missing 'rule_evaluation_latency_p99' query")
	}
}

func TestRulerBundle_UsesCorrectMetricNames(t *testing.T) {
	b := RulerBundle()
	byName := map[string]string{}
	for _, q := range b.Queries {
		byName[q.Name] = q.Expr
	}

	tests := []struct {
		name     string
		contains string
	}{
		{"rule_evaluation_latency_p99", "cortex_prometheus_rule_evaluation_duration_seconds_bucket"},
		{"missed_evaluations", "cortex_prometheus_rule_group_iterations_missed_total"},
		{"rules_per_group", "cortex_prometheus_rule_group_rules"},
		{"group_evaluation_duration_p99", "cortex_prometheus_rule_group_duration_seconds_bucket"},
		{"rule_evaluations_per_second", "cortex_prometheus_rule_evaluations_total"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, ok := byName[tt.name]
			if !ok {
				t.Fatalf("query %q not found", tt.name)
			}
			if !strings.Contains(expr, tt.contains) {
				t.Errorf("query %q expr = %q, want it to contain %q", tt.name, expr, tt.contains)
			}
		})
	}
}

func TestRulerBundle_NoLegacyMetricNames(t *testing.T) {
	b := RulerBundle()

	legacyNames := []string{
		"cortex_ruler_rule_evaluation_duration_seconds_bucket",
		"cortex_ruler_rule_evaluation_missed_iterations_total",
		"cortex_ruler_rule_group_rules",
		"cortex_ruler_rule_group_duration_seconds_bucket",
		"cortex_ruler_rule_evaluations_total",
	}

	for _, q := range b.Queries {
		for _, legacy := range legacyNames {
			if strings.Contains(q.Expr, legacy) {
				t.Errorf("query %q still contains legacy metric %q: %s", q.Name, legacy, q.Expr)
			}
		}
	}
}
