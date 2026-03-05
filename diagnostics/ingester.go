package diagnostics

func IngesterBundle() Bundle {
	return Bundle{
		Name:        "ingester",
		Description: "Ingester diagnostics — ingestion rate, ring health, WAL, memory",
		Queries: []DiagnosticQuery{
			{
				Name:        "ingestion_rate",
				Expr:        "rate(cortex_ingester_ingested_samples_total[5m])",
				Description: "Sample ingestion rate",
			},
			{
				Name:        "ingester_ring_members",
				Expr:        `cortex_ring_members{name="ingester"}`,
				Description: "Ingester ring health",
			},
			{
				Name:        "head_truncation_latency_p99",
				Expr:        "histogram_quantile(0.99, rate(cortex_ingester_tsdb_head_truncation_duration_seconds_bucket[5m]))",
				Description: "p99 WAL replay / flush latency",
			},
			{
				Name:        "active_series",
				Expr:        "cortex_ingester_memory_series",
				Description: "Active series per ingester",
			},
			{
				Name:        "memory_chunks",
				Expr:        "cortex_ingester_memory_chunks",
				Description: "Chunk utilization",
			},
		},
	}
}
