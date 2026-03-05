package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ampEndpointPattern = regexp.MustCompile(`^https://aps-workspaces\.[a-z0-9-]+\.amazonaws\.com/workspaces/ws-[a-zA-Z0-9-]+$`)

type Config struct {
	AMPEndpoint   string
	AWSRegion     string
	LoadtestStart time.Time
	LoadtestEnd   time.Time
	QueryTimeout  time.Duration
	MaxSeries     int
}

func Load() (*Config, error) {
	endpoint := os.Getenv("AMP_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("AMP_ENDPOINT is required")
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if !ampEndpointPattern.MatchString(endpoint) {
		return nil, fmt.Errorf("AMP_ENDPOINT has invalid format: %q — expected https://aps-workspaces.<region>.amazonaws.com/workspaces/ws-<id>", endpoint)
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		return nil, fmt.Errorf("AWS_REGION is required")
	}

	startStr := os.Getenv("LOADTEST_START")
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return nil, fmt.Errorf("LOADTEST_START must be RFC3339: %w", err)
	}

	endStr := os.Getenv("LOADTEST_END")
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return nil, fmt.Errorf("LOADTEST_END must be RFC3339: %w", err)
	}

	if !end.After(start) {
		return nil, fmt.Errorf("LOADTEST_END must be after LOADTEST_START")
	}

	timeout := 30 * time.Second
	if v := os.Getenv("QUERY_TIMEOUT_SECONDS"); v != "" {
		secs, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("QUERY_TIMEOUT_SECONDS must be an integer: %w", err)
		}
		timeout = time.Duration(secs) * time.Second
	}

	maxSeries := 2000
	if v := os.Getenv("MAX_SERIES_RETURNED"); v != "" {
		maxSeries, err = strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("MAX_SERIES_RETURNED must be an integer: %w", err)
		}
	}

	return &Config{
		AMPEndpoint:   endpoint,
		AWSRegion:     region,
		LoadtestStart: start,
		LoadtestEnd:   end,
		QueryTimeout:  timeout,
		MaxSeries:     maxSeries,
	}, nil
}

func (c *Config) ClampToWindow(start, end time.Time) (time.Time, time.Time) {
	if start.IsZero() || start.Before(c.LoadtestStart) {
		start = c.LoadtestStart
	}
	if end.IsZero() || end.After(c.LoadtestEnd) {
		end = c.LoadtestEnd
	}
	return start, end
}
