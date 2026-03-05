package diagnostics

func StoreGatewayBundle() Bundle {
	return Bundle{
		Name:        "store_gateway",
		Description: "Store Gateway diagnostics — block sync latency, cache hit rate",
		Queries: []DiagnosticQuery{
			{
				Name:        "series_fetch_latency_p99",
				Expr:        "histogram_quantile(0.99, rate(cortex_storegateway_series_fetch_duration_seconds_bucket[5m]))",
				Description: "p99 block sync / series fetch latency",
			},
			{
				Name:        "series_result_rate",
				Expr:        "rate(cortex_storegateway_series_result_series_total[5m])",
				Description: "Series result rate (cache hit indicator)",
			},
		},
	}
}
