package diagnostics

import (
	"context"
	"fmt"
	"time"

	"mimir-analyzer/amp"
	"mimir-analyzer/config"
)

type DiagnosticQuery struct {
	Name         string   `json:"name"`
	Expr         string   `json:"expr"`
	Description  string   `json:"description"`
	Alternatives []string `json:"alternatives,omitempty"`
}

type Bundle struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Queries     []DiagnosticQuery `json:"queries"`
}

type BundleResult struct {
	Subsystem string              `json:"subsystem"`
	Results   []QueryResultEntry  `json:"results"`
	Window    *amp.WindowInfo     `json:"window"`
}

type QueryResultEntry struct {
	Name    string           `json:"name"`
	Query   string           `json:"query"`
	Result  *amp.QueryResult `json:"result,omitempty"`
	Summary *QuerySummary    `json:"summary,omitempty"`
	Error   string           `json:"error,omitempty"`
}

var subsystemBundles = map[string]func() Bundle{
	"ruler":         RulerBundle,
	"ingester":      IngesterBundle,
	"querier":       QuerierBundle,
	"distributor":   DistributorBundle,
	"compactor":     CompactorBundle,
	"store_gateway": StoreGatewayBundle,
}

func GetBundle(subsystem string) ([]Bundle, error) {
	if subsystem == "all" {
		var bundles []Bundle
		for _, fn := range subsystemBundles {
			bundles = append(bundles, fn())
		}
		return bundles, nil
	}

	fn, ok := subsystemBundles[subsystem]
	if !ok {
		return nil, fmt.Errorf("unknown subsystem: %q (valid: ruler, ingester, querier, distributor, compactor, store_gateway, all)", subsystem)
	}
	return []Bundle{fn()}, nil
}

func RunBundle(ctx context.Context, client *amp.Client, cfg *config.Config, subsystem string, start, end time.Time) (*BundleResult, error) {
	bundles, err := GetBundle(subsystem)
	if err != nil {
		return nil, err
	}

	start, end = cfg.ClampToWindow(start, end)

	var results []QueryResultEntry
	for _, b := range bundles {
		for _, q := range b.Queries {
			entry := runQueryWithAlternatives(ctx, client, q, start, end)
			results = append(results, entry)
		}
	}

	return &BundleResult{
		Subsystem: subsystem,
		Results:   results,
		Window: &amp.WindowInfo{
			Start: start.Format(time.RFC3339),
			End:   end.Format(time.RFC3339),
		},
	}, nil
}

func runQueryWithAlternatives(ctx context.Context, client *amp.Client, q DiagnosticQuery, start, end time.Time) QueryResultEntry {
	entry := QueryResultEntry{Name: q.Name, Query: q.Expr}

	qr, err := client.QueryRange(ctx, q.Expr, start, end, "1m")
	if err != nil {
		entry.Error = err.Error()
		return entry
	}

	if qr.Status != "empty" || len(q.Alternatives) == 0 {
		entry.Result = qr
		entry.Summary, _ = ComputeSummary(qr.Data)
		return entry
	}

	// Primary returned empty, try alternatives
	for _, alt := range q.Alternatives {
		altResult, altErr := client.QueryRange(ctx, alt, start, end, "1m")
		if altErr != nil {
			continue
		}
		if altResult.Status != "empty" {
			entry.Query = alt
			entry.Result = altResult
			entry.Summary, _ = ComputeSummary(altResult.Data)
			return entry
		}
	}

	// All alternatives also empty, return the primary empty result
	entry.Result = qr
	entry.Summary, _ = ComputeSummary(qr.Data)
	return entry
}
