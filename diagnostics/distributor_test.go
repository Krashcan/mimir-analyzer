package diagnostics

import "testing"

func TestDistributorBundle_ReturnsExpectedQueries(t *testing.T) {
	b := DistributorBundle()
	if b.Name != "distributor" {
		t.Errorf("name = %q, want %q", b.Name, "distributor")
	}
	if len(b.Queries) != 3 {
		t.Errorf("got %d queries, want 3", len(b.Queries))
	}
	for _, q := range b.Queries {
		if q.Expr == "" {
			t.Errorf("query %q has empty expr", q.Name)
		}
	}
}
