package diagnostics

func DistributorBundle() Bundle {
	return Bundle{
		Name:        "distributor",
		Description: "Distributor diagnostics — receive latency, push failures, replication",
		Queries: []DiagnosticQuery{
			{
				Name:        "receive_latency_p99",
				Expr:        "histogram_quantile(0.99, rate(cortex_distributor_sample_delay_seconds_bucket[5m]))",
				Description: "p99 distributor receive latency",
			},
			{
				Name:        "push_failures",
				Expr:        "rate(cortex_distributor_receive_grpc_request_failures_total[5m])",
				Description: "Rate of push failures",
			},
			{
				Name:        "replication_failures",
				Expr:        "rate(cortex_distributor_replication_factor_failures_total[5m])",
				Description: "Rate of replication failures",
			},
		},
	}
}
