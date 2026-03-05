package diagnostics

import "testing"

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
