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

var ErrMCPMode = errors.New("no CLI subcommand: run in MCP mode")

func Run(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	if len(args) == 0 {
		return ErrMCPMode
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
		return fmt.Errorf("query failed: %w", err)
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
		return fmt.Errorf("query-range failed: %w", err)
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
		return fmt.Errorf("list-metrics failed: %w", err)
	}
	return writeJSON(w, result)
}

func runDiagnose(ctx context.Context, args []string, client *amp.Client, cfg *config.Config, w io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mimir-analyzer diagnose <subsystem>")
	}
	subsystem := args[0]

	result, err := diagnostics.RunBundle(ctx, client, cfg, subsystem, time.Time{}, time.Time{})
	if err != nil {
		return fmt.Errorf("diagnose failed: %w", err)
	}
	return writeJSON(w, result)
}

func runCheckConnection(ctx context.Context, client *amp.Client, w io.Writer) error {
	status := client.CheckConnection(ctx)
	return writeJSON(w, status)
}
