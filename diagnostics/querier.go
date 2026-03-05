package diagnostics

func QuerierBundle() Bundle {
	return Bundle{
		Name:        "querier",
		Description: "Querier / Query Frontend diagnostics — latency, queue depth, errors",
		Queries: []DiagnosticQuery{
			{
				Name:        "query_latency_p99",
				Expr:        "histogram_quantile(0.99, rate(cortex_querier_query_duration_seconds_bucket[5m]))",
				Description: "p99 query latency (ruler queries go through querier)",
			},
			{
				Name:        "query_frontend_queue_length",
				Expr:        "cortex_query_frontend_queue_length",
				Description: "Queue depth at query frontend",
			},
			{
				Name:        "querier_errors",
				Expr:        "rate(cortex_querier_queries_failed_total[5m])",
				Description: "Rate of querier errors",
			},
			{
				Name:        "query_frontend_retries",
				Expr:        "rate(cortex_query_frontend_retries_total[5m])",
				Description: "Rate of query retries",
			},
		},
	}
}
