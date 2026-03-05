package amp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

func staticCreds() aws.CredentialsProvider {
	return aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			SessionToken:    "test-session-token",
		}, nil
	})
}

func newSigV4Client(t *testing.T, serverURL string) *http.Client {
	t.Helper()
	rt := &sigV4RoundTripper{
		signer: v4.NewSigner(),
		creds:  staticCreds(),
		region: "us-east-1",
		next:   http.DefaultTransport,
	}
	return &http.Client{Transport: rt}
}

func TestSigV4RoundTripper_SignsRequests(t *testing.T) {
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query?query=up", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if capturedAuth == "" {
		t.Fatal("Authorization header was not set")
	}
	if !strings.Contains(capturedAuth, "AWS4-HMAC-SHA256") {
		t.Errorf("Authorization header does not contain SigV4 signature: %q", capturedAuth)
	}
}

func TestSigV4RoundTripper_401ReturnsCredentialError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "credentials expired or invalid") {
		t.Errorf("error should mention credentials: %v", err)
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatal("error should be *AMPError")
	}
	if ampErr.Category != CategoryAuth {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryAuth)
	}
}

func TestSigV4RoundTripper_403ReturnsCredentialError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "credentials expired or invalid") {
		t.Errorf("error should mention credentials: %v", err)
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatal("error should be *AMPError")
	}
	if ampErr.Category != CategoryAuth {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryAuth)
	}
}

func TestSigV4RoundTripper_5xxReturnsWrappedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code: %v", err)
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatal("error should be *AMPError")
	}
	if ampErr.Category != CategoryServerError {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryServerError)
	}
}

func TestSigV4RoundTripper_400ReturnsBadRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"status":"error","errorType":"bad_data","error":"invalid expression"}`)
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query?query=bad(", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected error for 400")
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatal("error should be *AMPError")
	}
	if ampErr.Category != CategoryQueryError {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryQueryError)
	}
	if ampErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", ampErr.StatusCode)
	}
}

func TestSigV4RoundTripper_404ReturnsConfigError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "<html>Not Found</html>")
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected error for 404")
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatal("error should be *AMPError")
	}
	if ampErr.Category != CategoryConfigError {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryConfigError)
	}
	if !strings.Contains(ampErr.Message, "AMP_ENDPOINT") {
		t.Errorf("message should mention AMP_ENDPOINT: %q", ampErr.Message)
	}
}

func TestSigV4RoundTripper_429ReturnsRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, "rate limit exceeded")
	}))
	defer server.Close()

	client := newSigV4Client(t, server.URL)
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/query", nil)
	_, err := client.Do(req)

	if err == nil {
		t.Fatal("expected error for 429")
	}

	var ampErr *AMPError
	if !errors.As(err, &ampErr) {
		t.Fatal("error should be *AMPError")
	}
	if ampErr.Category != CategoryServerError {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryServerError)
	}
	if !strings.Contains(ampErr.Message, "rate limited") {
		t.Errorf("message should mention rate limiting: %q", ampErr.Message)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("https://aps-workspaces.us-east-1.amazonaws.com/workspaces/ws-xxx", "us-east-1", staticCreds(), 30*time.Second)
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestCheckConnection_Connected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status/buildinfo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"success","data":{"version":"2.10.0","branch":"HEAD"}}`)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	status := c.CheckConnection(context.Background())

	if status.Status != "connected" {
		t.Errorf("status = %q, want %q", status.Status, "connected")
	}
	if status.Endpoint != server.URL {
		t.Errorf("endpoint = %q, want %q", status.Endpoint, server.URL)
	}
	if status.BuildInfo == nil {
		t.Error("build_info should not be nil when connected")
	}
}

func TestCheckConnection_AuthFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	status := c.CheckConnection(context.Background())

	if status.Status != "auth_failed" {
		t.Errorf("status = %q, want %q", status.Status, "auth_failed")
	}
}

func TestCheckConnection_Unreachable(t *testing.T) {
	c := newTestClient("http://127.0.0.1:1")
	status := c.CheckConnection(context.Background())

	if status.Status != "unreachable" {
		t.Errorf("status = %q, want %q", status.Status, "unreachable")
	}
}

func TestCheckConnection_WrongEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "<html>Not Found</html>")
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	status := c.CheckConnection(context.Background())

	if status.Status != "wrong_endpoint" {
		t.Errorf("status = %q, want %q", status.Status, "wrong_endpoint")
	}
}

func TestCheckConnection_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	c := newTestClient(server.URL)
	status := c.CheckConnection(context.Background())

	if status.Status != "error" {
		t.Errorf("status = %q, want %q", status.Status, "error")
	}
}
