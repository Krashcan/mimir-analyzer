package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// helper to set env vars and return a cleanup function
func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"AMP_ENDPOINT":   "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxx",
		"AWS_REGION":     "us-east-1",
		"LOADTEST_START": "2024-01-15T10:00:00Z",
		"LOADTEST_END":   "2024-01-15T12:00:00Z",
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AMPEndpoint != "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxx" {
		t.Errorf("AMPEndpoint = %q, want the endpoint URL", cfg.AMPEndpoint)
	}
	if cfg.AWSRegion != "us-east-1" {
		t.Errorf("AWSRegion = %q, want us-east-1", cfg.AWSRegion)
	}
	expectedStart := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !cfg.LoadtestStart.Equal(expectedStart) {
		t.Errorf("LoadtestStart = %v, want %v", cfg.LoadtestStart, expectedStart)
	}
	expectedEnd := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	if !cfg.LoadtestEnd.Equal(expectedEnd) {
		t.Errorf("LoadtestEnd = %v, want %v", cfg.LoadtestEnd, expectedEnd)
	}
}

func TestLoad_MissingAMPEndpoint(t *testing.T) {
	env := validEnv()
	delete(env, "AMP_ENDPOINT")
	setEnv(t, env)
	os.Unsetenv("AMP_ENDPOINT")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing AMP_ENDPOINT")
	}
}

func TestLoad_MissingAWSRegion(t *testing.T) {
	env := validEnv()
	delete(env, "AWS_REGION")
	setEnv(t, env)
	os.Unsetenv("AWS_REGION")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing AWS_REGION")
	}
}

func TestLoad_MalformedLoadtestStart(t *testing.T) {
	env := validEnv()
	env["LOADTEST_START"] = "not-a-date"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed LOADTEST_START")
	}
}

func TestLoad_MalformedLoadtestEnd(t *testing.T) {
	env := validEnv()
	env["LOADTEST_END"] = "not-a-date"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for malformed LOADTEST_END")
	}
}

func TestLoad_EndBeforeStart(t *testing.T) {
	env := validEnv()
	env["LOADTEST_START"] = "2024-01-15T12:00:00Z"
	env["LOADTEST_END"] = "2024-01-15T10:00:00Z"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when LOADTEST_END is before LOADTEST_START")
	}
}

func TestLoad_DefaultQueryTimeout(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.QueryTimeout != 30*time.Second {
		t.Errorf("QueryTimeout = %v, want 30s", cfg.QueryTimeout)
	}
}

func TestLoad_DefaultMaxSeries(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxSeries != 2000 {
		t.Errorf("MaxSeries = %d, want 2000", cfg.MaxSeries)
	}
}

func TestLoad_CustomQueryTimeout(t *testing.T) {
	env := validEnv()
	env["QUERY_TIMEOUT_SECONDS"] = "60"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.QueryTimeout != 60*time.Second {
		t.Errorf("QueryTimeout = %v, want 60s", cfg.QueryTimeout)
	}
}

func TestLoad_CustomMaxSeries(t *testing.T) {
	env := validEnv()
	env["MAX_SERIES_RETURNED"] = "500"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxSeries != 500 {
		t.Errorf("MaxSeries = %d, want 500", cfg.MaxSeries)
	}
}

func TestLoad_InvalidAMPEndpoint_MissingWorkspacePath(t *testing.T) {
	env := validEnv()
	env["AMP_ENDPOINT"] = "https://aps-workspaces.us-east-1.amazonaws.com"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for AMP_ENDPOINT missing /workspaces/ path")
	}
	if !strings.Contains(err.Error(), "AMP_ENDPOINT") {
		t.Errorf("error should mention AMP_ENDPOINT: %v", err)
	}
}

func TestLoad_InvalidAMPEndpoint_WrongHost(t *testing.T) {
	env := validEnv()
	env["AMP_ENDPOINT"] = "https://prometheus.example.com/workspaces/ws-xxx"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for AMP_ENDPOINT with wrong host")
	}
}

func TestLoad_InvalidAMPEndpoint_MissingWSPrefix(t *testing.T) {
	env := validEnv()
	env["AMP_ENDPOINT"] = "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/abc123"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for workspace ID missing ws- prefix")
	}
}

func TestLoad_InvalidAMPEndpoint_HTTPScheme(t *testing.T) {
	env := validEnv()
	env["AMP_ENDPOINT"] = "http://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxx"
	setEnv(t, env)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for http:// scheme (must be https://)")
	}
}

func TestLoad_ValidAMPEndpoint_TrailingSlash(t *testing.T) {
	env := validEnv()
	env["AMP_ENDPOINT"] = "https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxx/"
	setEnv(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasSuffix(cfg.AMPEndpoint, "/") {
		t.Errorf("AMPEndpoint should have trailing slash stripped, got %q", cfg.AMPEndpoint)
	}
}

func TestClampToWindow_BothWithin(t *testing.T) {
	cfg := &Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	start := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 11, 30, 0, 0, time.UTC)

	s, e := cfg.ClampToWindow(start, end)
	if !s.Equal(start) {
		t.Errorf("start = %v, want %v", s, start)
	}
	if !e.Equal(end) {
		t.Errorf("end = %v, want %v", e, end)
	}
}

func TestClampToWindow_StartBeforeWindow(t *testing.T) {
	cfg := &Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	start := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

	s, e := cfg.ClampToWindow(start, end)
	if !s.Equal(cfg.LoadtestStart) {
		t.Errorf("start = %v, want clamped to %v", s, cfg.LoadtestStart)
	}
	if !e.Equal(end) {
		t.Errorf("end = %v, want %v", e, end)
	}
}

func TestClampToWindow_EndAfterWindow(t *testing.T) {
	cfg := &Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}
	start := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)

	s, e := cfg.ClampToWindow(start, end)
	if !s.Equal(start) {
		t.Errorf("start = %v, want %v", s, start)
	}
	if !e.Equal(cfg.LoadtestEnd) {
		t.Errorf("end = %v, want clamped to %v", e, cfg.LoadtestEnd)
	}
}

func TestClampToWindow_BothZero(t *testing.T) {
	cfg := &Config{
		LoadtestStart: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LoadtestEnd:   time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	s, e := cfg.ClampToWindow(time.Time{}, time.Time{})
	if !s.Equal(cfg.LoadtestStart) {
		t.Errorf("start = %v, want %v", s, cfg.LoadtestStart)
	}
	if !e.Equal(cfg.LoadtestEnd) {
		t.Errorf("end = %v, want %v", e, cfg.LoadtestEnd)
	}
}
