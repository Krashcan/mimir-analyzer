package diagnostics

func RulerBundle() Bundle {
	return Bundle{
		Name:        "ruler",
		Description: "Ruler diagnostics — rule evaluation, missed iterations, scheduling",
		Queries: []DiagnosticQuery{
			{
				Name:        "rule_evaluation_latency_p99",
				Expr:        "histogram_quantile(0.99, rate(cortex_ruler_rule_evaluation_duration_seconds_bucket[5m]))",
				Description: "p99 rule evaluation latency",
			},
			{
				Name:        "missed_evaluations",
				Expr:        "increase(cortex_ruler_rule_evaluation_missed_iterations_total[5m])",
				Description: "Missed rule evaluations — the primary symptom",
			},
			{
				Name:        "rules_per_group",
				Expr:        "cortex_ruler_rule_group_rules",
				Description: "Number of rules per rule group (high counts = slow evaluation)",
			},
			{
				Name:        "group_evaluation_duration_p99",
				Expr:        "histogram_quantile(0.99, rate(cortex_ruler_rule_group_duration_seconds_bucket[5m]))",
				Description: "p99 full group evaluation duration",
			},
			{
				Name:        "querier_errors_from_ruler",
				Expr:        "rate(cortex_ruler_queries_failed_total[5m])",
				Description: "Rate of querier errors originating from ruler",
			},
			{
				Name:        "ruler_ring_members",
				Expr:        `cortex_ring_members{name="ruler"}`,
				Description: "Ruler ring status — check for unhealthy instances",
			},
			{
				Name:        "rule_evaluations_per_second",
				Expr:        "rate(cortex_ruler_rule_evaluations_total[5m])",
				Description: "Rule evaluations per second",
			},
			{
				Name:        "ruler_sync_rules",
				Expr:        "cortex_ruler_sync_rules_total",
				Description: "Scheduler queue depth for rule syncing",
			},
		},
	}
}
