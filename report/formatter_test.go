package report

import (
	"strings"
	"testing"
)

func TestFormatReport_ProducesValidMarkdown(t *testing.T) {
	findings := []Finding{
		{
			Component:   "ruler",
			Metric:      "cortex_ruler_rule_evaluation_missed_iterations_total",
			Value:       "1523",
			Description: "Missed evaluations during load test",
			Severity:    "high",
		},
		{
			Component:   "querier",
			Metric:      "cortex_query_frontend_queue_length",
			Value:       "42",
			Description: "Query frontend queue backed up",
			Severity:    "medium",
		},
	}

	report := FormatReport("261k alerts, 10:00-12:00 UTC", findings)

	if !strings.Contains(report, "# Mimir Load Test Bottleneck Analysis") {
		t.Error("report missing main header")
	}
	if !strings.Contains(report, "ruler") {
		t.Error("report missing ruler finding")
	}
	if !strings.Contains(report, "querier") {
		t.Error("report missing querier finding")
	}
	if !strings.Contains(report, "1523") {
		t.Error("report missing metric value")
	}
	// Check it has markdown table markers
	if !strings.Contains(report, "|") {
		t.Error("report missing markdown table formatting")
	}
}

func TestFormatReport_EmptyFindings(t *testing.T) {
	report := FormatReport("261k alerts, 10:00-12:00 UTC", nil)

	if !strings.Contains(report, "# Mimir Load Test Bottleneck Analysis") {
		t.Error("report missing main header")
	}
	if !strings.Contains(report, "No findings") {
		t.Error("empty report should contain 'No findings' message")
	}
}
