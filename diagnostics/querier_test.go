package diagnostics

import "testing"

func TestQuerierBundle_ReturnsExpectedQueries(t *testing.T) {
	b := QuerierBundle()
	if b.Name != "querier" {
		t.Errorf("name = %q, want %q", b.Name, "querier")
	}
	if len(b.Queries) != 4 {
		t.Errorf("got %d queries, want 4", len(b.Queries))
	}
	for _, q := range b.Queries {
		if q.Expr == "" {
			t.Errorf("query %q has empty expr", q.Name)
		}
	}
}
