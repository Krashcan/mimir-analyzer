package diagnostics

import "testing"

func TestIngesterBundle_ReturnsExpectedQueries(t *testing.T) {
	b := IngesterBundle()
	if b.Name != "ingester" {
		t.Errorf("name = %q, want %q", b.Name, "ingester")
	}
	if len(b.Queries) != 5 {
		t.Errorf("got %d queries, want 5", len(b.Queries))
	}
	for _, q := range b.Queries {
		if q.Expr == "" {
			t.Errorf("query %q has empty expr", q.Name)
		}
	}
}
