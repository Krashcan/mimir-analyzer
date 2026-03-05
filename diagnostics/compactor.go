package diagnostics

func CompactorBundle() Bundle {
	return Bundle{
		Name:        "compactor",
		Description: "Compactor diagnostics — compaction runs and failures",
		Queries: []DiagnosticQuery{
			{
				Name:        "compaction_runs",
				Expr:        "rate(cortex_compactor_runs_started_total[5m])",
				Description: "Rate of compaction runs started",
			},
			{
				Name:        "compaction_failures",
				Expr:        "rate(cortex_compactor_runs_failed_total[5m])",
				Description: "Rate of compaction failures",
			},
		},
	}
}
