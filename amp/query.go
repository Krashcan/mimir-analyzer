package amp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// QueryResult is the structured response from instant and range queries.
type QueryResult struct {
	Status  string          `json:"status"`
	Query   string          `json:"query"`
	Data    json.RawMessage `json:"data,omitempty"`
	Summary string          `json:"summary,omitempty"`
	Window  *WindowInfo     `json:"window,omitempty"`
}

type WindowInfo struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// prometheusResponse is the raw Prometheus HTTP API response.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string            `json:"resultType"`
		Result     json.RawMessage   `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

func (c *Client) QueryInstant(ctx context.Context, expr string, t time.Time) (*QueryResult, error) {
	if c.config != nil {
		t, _ = c.config.ClampToWindow(t, t)
	}

	params := url.Values{}
	params.Set("query", expr)
	params.Set("time", t.Format(time.RFC3339))

	reqURL := fmt.Sprintf("%s/api/v1/query?%s", c.endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ClassifyError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var promResp prometheusResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("query failed: %s (%s)", promResp.Error, promResp.ErrorType)
	}

	status := "ok"
	if isEmptyResult(promResp.Data.Result) {
		status = "empty"
	}

	result := &QueryResult{
		Status: status,
		Query:  expr,
		Data:   promResp.Data.Result,
	}
	if c.config != nil {
		result.Window = &WindowInfo{
			Start: c.config.LoadtestStart.Format(time.RFC3339),
			End:   c.config.LoadtestEnd.Format(time.RFC3339),
		}
	}
	return result, nil
}

func (c *Client) QueryRange(ctx context.Context, expr string, start, end time.Time, step string) (*QueryResult, error) {
	if c.config != nil {
		start, end = c.config.ClampToWindow(start, end)
	}

	params := url.Values{}
	params.Set("query", expr)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", end.Format(time.RFC3339))
	params.Set("step", step)

	reqURL := fmt.Sprintf("%s/api/v1/query_range?%s", c.endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ClassifyError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var promResp prometheusResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("query failed: %s (%s)", promResp.Error, promResp.ErrorType)
	}

	status := "ok"
	if isEmptyResult(promResp.Data.Result) {
		status = "empty"
	}

	result := &QueryResult{
		Status: status,
		Query:  expr,
		Data:   promResp.Data.Result,
	}
	if c.config != nil {
		result.Window = &WindowInfo{
			Start: c.config.LoadtestStart.Format(time.RFC3339),
			End:   c.config.LoadtestEnd.Format(time.RFC3339),
		}
	}
	return result, nil
}

func isEmptyResult(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return false
	}
	return len(arr) == 0
}
