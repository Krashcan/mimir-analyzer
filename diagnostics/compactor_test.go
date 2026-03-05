package diagnostics

import "testing"

func TestCompactorBundle_ReturnsExpectedQueries(t *testing.T) {
	b := CompactorBundle()
	if b.Name != "compactor" {
		t.Errorf("name = %q, want %q", b.Name, "compactor")
	}
	if len(b.Queries) != 2 {
		t.Errorf("got %d queries, want 2", len(b.Queries))
	}
	for _, q := range b.Queries {
		if q.Expr == "" {
			t.Errorf("query %q has empty expr", q.Name)
		}
	}
}
