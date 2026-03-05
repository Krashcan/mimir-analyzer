package amp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"mimir-analyzer/config"
)

type Client struct {
	endpoint   string
	httpClient *http.Client
	config     *config.Config
}

type sigV4RoundTripper struct {
	signer *v4.Signer
	creds  aws.CredentialsProvider
	region string
	next   http.RoundTripper
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func (t *sigV4RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	hash := sha256Hex(bodyBytes)

	creds, err := t.creds.Retrieve(req.Context())
	if err != nil {
		return nil, fmt.Errorf("retrieving AWS credentials: %w", err)
	}

	err = t.signer.SignHTTP(req.Context(), creds, req, hash, "aps", t.region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("signing request: %w", err)
	}

	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, classifyHTTPError(resp.StatusCode, body)
	}

	return resp, nil
}

// classifyHTTPError maps HTTP status codes to categorized *AMPError values.
func classifyHTTPError(statusCode int, body []byte) *AMPError {
	snippet := string(body)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}

	switch {
	case statusCode == 401 || statusCode == 403:
		return &AMPError{
			Category:   CategoryAuth,
			Message:    fmt.Sprintf("AWS credentials expired or invalid (HTTP %d) — ensure valid credentials are present in your environment", statusCode),
			StatusCode: statusCode,
		}
	case statusCode == 404:
		return &AMPError{
			Category:   CategoryConfigError,
			Message:    fmt.Sprintf("AMP endpoint not found (HTTP 404) — check AMP_ENDPOINT is correct: %s", snippet),
			StatusCode: statusCode,
		}
	case statusCode == 429:
		return &AMPError{
			Category:   CategoryServerError,
			Message:    fmt.Sprintf("rate limited by AMP (HTTP 429): %s", snippet),
			StatusCode: statusCode,
		}
	case statusCode == 400:
		return &AMPError{
			Category:   CategoryQueryError,
			Message:    fmt.Sprintf("bad request (HTTP 400): %s", snippet),
			StatusCode: statusCode,
		}
	case statusCode >= 400 && statusCode < 500:
		return &AMPError{
			Category:   CategoryQueryError,
			Message:    fmt.Sprintf("client error (HTTP %d): %s", statusCode, snippet),
			StatusCode: statusCode,
		}
	default:
		return &AMPError{
			Category:   CategoryServerError,
			Message:    fmt.Sprintf("AMP server error (HTTP %d): %s", statusCode, snippet),
			StatusCode: statusCode,
		}
	}
}

func NewClient(endpoint, region string, creds aws.CredentialsProvider, timeout time.Duration) *Client {
	rt := &sigV4RoundTripper{
		signer: v4.NewSigner(),
		creds:  creds,
		region: region,
		next:   http.DefaultTransport,
	}

	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Transport: rt,
			Timeout:   timeout,
		},
	}
}

func NewClientWithConfig(cfg *config.Config, creds aws.CredentialsProvider) *Client {
	c := NewClient(cfg.AMPEndpoint, cfg.AWSRegion, creds, cfg.QueryTimeout)
	c.config = cfg
	return c
}

// NewTestClient creates a Client without SigV4 for use in tests.
func NewTestClient(endpoint string, cfg *config.Config) *Client {
	return &Client{
		endpoint:   endpoint,
		httpClient: &http.Client{},
		config:     cfg,
	}
}

// ConnectionStatus is the structured result of a connection check.
type ConnectionStatus struct {
	Status    string `json:"status"`
	Endpoint  string `json:"endpoint"`
	Message   string `json:"message"`
	BuildInfo any    `json:"build_info,omitempty"`
}

// CheckConnection verifies connectivity to the AMP endpoint by calling /api/v1/status/buildinfo.
// It always returns a ConnectionStatus (never a Go error) so the handler can always return structured JSON.
func (c *Client) CheckConnection(ctx context.Context) *ConnectionStatus {
	reqURL := fmt.Sprintf("%s/api/v1/status/buildinfo", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return &ConnectionStatus{
			Status:   "error",
			Endpoint: c.endpoint,
			Message:  "failed to create request: " + err.Error(),
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Classify the error to determine the right status
		var ampErr *AMPError
		if errors.As(err, &ampErr) {
			switch ampErr.Category {
			case CategoryAuth:
				return &ConnectionStatus{
					Status:   "auth_failed",
					Endpoint: c.endpoint,
					Message:  ampErr.Message,
				}
			case CategoryConnectivity:
				return &ConnectionStatus{
					Status:   "unreachable",
					Endpoint: c.endpoint,
					Message:  ampErr.Message,
				}
			case CategoryConfigError:
				return &ConnectionStatus{
					Status:   "wrong_endpoint",
					Endpoint: c.endpoint,
					Message:  ampErr.Message,
				}
			}
		}
		classified := ClassifyError(err)
		switch classified.Category {
		case CategoryConnectivity:
			return &ConnectionStatus{
				Status:   "unreachable",
				Endpoint: c.endpoint,
				Message:  classified.Message,
			}
		case CategoryAuth:
			return &ConnectionStatus{
				Status:   "auth_failed",
				Endpoint: c.endpoint,
				Message:  classified.Message,
			}
		default:
			return &ConnectionStatus{
				Status:   "error",
				Endpoint: c.endpoint,
				Message:  classified.Message,
			}
		}
	}
	defer resp.Body.Close()

	// Handle non-200 responses for test clients (where round tripper doesn't intercept)
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return &ConnectionStatus{
			Status:   "auth_failed",
			Endpoint: c.endpoint,
			Message:  fmt.Sprintf("AWS credentials expired or invalid (HTTP %d)", resp.StatusCode),
		}
	}
	if resp.StatusCode == 404 {
		return &ConnectionStatus{
			Status:   "wrong_endpoint",
			Endpoint: c.endpoint,
			Message:  "AMP endpoint not found (HTTP 404) — check AMP_ENDPOINT is correct",
		}
	}
	if resp.StatusCode >= 400 {
		return &ConnectionStatus{
			Status:   "error",
			Endpoint: c.endpoint,
			Message:  fmt.Sprintf("AMP returned HTTP %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ConnectionStatus{
			Status:   "error",
			Endpoint: c.endpoint,
			Message:  "failed to read response: " + err.Error(),
		}
	}

	var buildInfo any
	if err := json.Unmarshal(body, &buildInfo); err != nil {
		return &ConnectionStatus{
			Status:   "connected",
			Endpoint: c.endpoint,
			Message:  "connected (could not parse build info)",
		}
	}

	return &ConnectionStatus{
		Status:    "connected",
		Endpoint:  c.endpoint,
		Message:   "successfully connected to AMP",
		BuildInfo: buildInfo,
	}
}
