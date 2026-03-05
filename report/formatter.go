package report

import (
	"fmt"
	"strings"
)

type Finding struct {
	Component   string `json:"component"`
	Metric      string `json:"metric"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

func FormatReport(testParams string, findings []Finding) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Mimir Load Test Bottleneck Analysis\n")
	fmt.Fprintf(&b, "**Test parameters:** %s\n\n", testParams)

	if len(findings) == 0 {
		fmt.Fprintf(&b, "## Findings\n\n")
		fmt.Fprintf(&b, "No findings — no anomalies detected in the queried metrics.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "## Findings\n\n")
	fmt.Fprintf(&b, "| Component | Metric | Value | Severity | Description |\n")
	fmt.Fprintf(&b, "|-----------|--------|-------|----------|-------------|\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			f.Component, f.Metric, f.Value, f.Severity, f.Description)
	}

	return b.String()
}
