package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"mimir-analyzer/amp"
	"mimir-analyzer/config"
	"mimir-analyzer/diagnostics"
)

const usageText = `USAGE
  mimir-analyzer <command> [options]
  mimir-analyzer --help

COMMANDS
  query <expr> [--time <RFC3339>]                   Run an instant PromQL query
  query-range <expr> [--step 5m] [--start/--end]    Run a range query
  list-metrics [--match '{...}'] [--limit N]        List available metrics
  diagnose <subsystem> [--start/--end] [--verbose]  Run diagnostic bundle
  check-connection                                  Verify AMP connectivity

Run 'mimir-analyzer --help' for full documentation.
`

func Run(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(w, usageText)
		return nil
	}

	switch args[0] {
	case "query":
		return runQuery(ctx, args[1:], client, cfg, w)
	case "query-range":
		return runQueryRange(ctx, args[1:], client, cfg, w)
	case "list-metrics":
		return runListMetrics(ctx, args[1:], client, cfg, w)
	case "diagnose":
		return runDiagnose(ctx, args[1:], client, cfg, w)
	case "check-connection":
		return runCheckConnection(ctx, client, w)
	default:
		return fmt.Errorf("unknown subcommand: %q (valid: query, query-range, list-metrics, diagnose, check-connection)", args[0])
	}
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// errorResponse is a structured JSON error returned to the caller.
type errorResponse struct {
	Status   string `json:"status"`
	Error    string `json:"error"`
	Category string `json:"category,omitempty"`
}

// writeError writes a structured JSON error response. If the error is an *amp.AMPError,
// the category is extracted and included.
func writeError(w io.Writer, prefix string, err error) error {
	resp := errorResponse{Status: "error"}
	var ampErr *amp.AMPError
	if errors.As(err, &ampErr) {
		resp.Category = string(ampErr.Category)
		resp.Error = prefix + ampErr.Message
	} else {
		resp.Error = prefix + err.Error()
	}
	return writeJSON(w, resp)
}

func runQuery(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mimir-analyzer query <expr> [--time <RFC3339>]")
	}
	expr := args[0]

	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	timeStr := fs.String("time", "", "RFC3339 timestamp (defaults to LOADTEST_END)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	queryTime := cfg.LoadtestEnd
	if *timeStr != "" {
		var err error
		queryTime, err = time.Parse(time.RFC3339, *timeStr)
		if err != nil {
			return fmt.Errorf("invalid --time: %w", err)
		}
	}

	result, err := client.QueryInstant(ctx, expr, queryTime)
	if err != nil {
		return writeError(w, "query failed: ", err)
	}
	return writeJSON(w, result)
}

func runQueryRange(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mimir-analyzer query-range <expr> [--step <duration>] [--start <RFC3339>] [--end <RFC3339>]")
	}
	expr := args[0]

	fs := flag.NewFlagSet("query-range", flag.ContinueOnError)
	step := fs.String("step", "1m", "Query step duration")
	startStr := fs.String("start", "", "RFC3339 start time")
	endStr := fs.String("end", "", "RFC3339 end time")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	var start, end time.Time
	if *startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, *startStr)
		if err != nil {
			return fmt.Errorf("invalid --start: %w", err)
		}
	}
	if *endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, *endStr)
		if err != nil {
			return fmt.Errorf("invalid --end: %w", err)
		}
	}

	result, err := client.QueryRange(ctx, expr, start, end, *step)
	if err != nil {
		return writeError(w, "query-range failed: ", err)
	}
	return writeJSON(w, result)
}

func runListMetrics(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	fs := flag.NewFlagSet("list-metrics", flag.ContinueOnError)
	match := fs.String("match", "", "Label selector filter")
	limit := fs.Int("limit", 200, "Max results")
	if err := fs.Parse(args); err != nil {
		return err
	}

	result, err := client.ListMetrics(ctx, *match, *limit)
	if err != nil {
		return writeError(w, "list-metrics failed: ", err)
	}
	return writeJSON(w, result)
}

func runDiagnose(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mimir-analyzer diagnose <subsystem> [--start <RFC3339>] [--end <RFC3339>] [--verbose]")
	}
	subsystem := args[0]

	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	startStr := fs.String("start", "", "RFC3339 start time (narrowing within load test window)")
	endStr := fs.String("end", "", "RFC3339 end time (narrowing within load test window)")
	verbose := fs.Bool("verbose", false, "Include raw query data in results")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	var start, end time.Time
	if *startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, *startStr)
		if err != nil {
			return fmt.Errorf("invalid --start: %w", err)
		}
	}
	if *endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, *endStr)
		if err != nil {
			return fmt.Errorf("invalid --end: %w", err)
		}
	}

	result, err := diagnostics.RunBundle(ctx, client, cfg, subsystem, start, end)
	if err != nil {
		return writeError(w, "", err)
	}

	if !*verbose {
		for i := range result.Results {
			if result.Results[i].Result != nil {
				result.Results[i].Result.Data = nil
			}
		}
	}

	return writeJSON(w, result)
}

func runCheckConnection(ctx context.Context, client *amp.Client, w io.Writer) error {
	status := client.CheckConnection(ctx)
	return writeJSON(w, status)
}
