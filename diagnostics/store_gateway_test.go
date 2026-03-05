package diagnostics

import "testing"

func TestStoreGatewayBundle_ReturnsExpectedQueries(t *testing.T) {
	b := StoreGatewayBundle()
	if b.Name != "store_gateway" {
		t.Errorf("name = %q, want %q", b.Name, "store_gateway")
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
