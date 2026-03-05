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

type ListMetricsResult struct {
	Status    string   `json:"status"`
	Data      []string `json:"data"`
	Truncated bool     `json:"truncated,omitempty"`
	Window    *WindowInfo `json:"window,omitempty"`
}

func (c *Client) ListMetrics(ctx context.Context, match string, limit int) (*ListMetricsResult, error) {
	if limit <= 0 {
		limit = 200
	}

	params := url.Values{}
	if match != "" {
		params.Set("match[]", match)
	}
	if c.config != nil {
		params.Set("start", c.config.LoadtestStart.Format(time.RFC3339))
		params.Set("end", c.config.LoadtestEnd.Format(time.RFC3339))
	}

	reqURL := fmt.Sprintf("%s/api/v1/label/__name__/values?%s", c.endpoint, params.Encode())
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

	var promResp struct {
		Status    string   `json:"status"`
		Data      []string `json:"data"`
		Error     string   `json:"error,omitempty"`
		ErrorType string   `json:"errorType,omitempty"`
	}
	if err := json.Unmarshal(body, &promResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("list_metrics failed: %s (%s)", promResp.Error, promResp.ErrorType)
	}

	truncated := false
	data := promResp.Data
	if len(data) > limit {
		data = data[:limit]
		truncated = true
	}

	result := &ListMetricsResult{
		Status:    "ok",
		Data:      data,
		Truncated: truncated,
	}
	if c.config != nil {
		result.Window = &WindowInfo{
			Start: c.config.LoadtestStart.Format(time.RFC3339),
			End:   c.config.LoadtestEnd.Format(time.RFC3339),
		}
	}
	return result, nil
}
