package amp

import (
	"context"
	"errors"
	"net"
	"strings"
)

// ErrorCategory classifies AMP errors so the LLM can reason about what went wrong.
type ErrorCategory string

const (
	CategoryConnectivity ErrorCategory = "connectivity"
	CategoryAuth         ErrorCategory = "auth"
	CategoryTimeout      ErrorCategory = "timeout"
	CategoryQueryError   ErrorCategory = "query_error"
	CategoryConfigError  ErrorCategory = "config_error"
	CategoryServerError  ErrorCategory = "server_error"
)

// AMPError is a structured error returned from AMP operations.
type AMPError struct {
	Category   ErrorCategory
	Message    string
	StatusCode int
	Cause      error
}

func (e *AMPError) Error() string {
	return e.Message
}

func (e *AMPError) Unwrap() error {
	return e.Cause
}

// ClassifyError inspects an error and wraps it in an *AMPError with the appropriate category.
func ClassifyError(err error) *AMPError {
	if err == nil {
		return nil
	}

	// Already classified
	var ampErr *AMPError
	if errors.As(err, &ampErr) {
		return ampErr
	}

	// DNS failure
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return &AMPError{
			Category: CategoryConnectivity,
			Message:  "DNS resolution failed: " + dnsErr.Error(),
			Cause:    err,
		}
	}

	// Connection refused / network operation error
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if strings.Contains(opErr.Error(), "connection refused") {
			return &AMPError{
				Category: CategoryConnectivity,
				Message:  "connection refused: " + opErr.Error(),
				Cause:    err,
			}
		}
	}

	// Timeout
	if errors.Is(err, context.DeadlineExceeded) {
		return &AMPError{
			Category: CategoryTimeout,
			Message:  "request timed out: " + err.Error(),
			Cause:    err,
		}
	}

	// Fallback
	return &AMPError{
		Category: CategoryServerError,
		Message:  err.Error(),
		Cause:    err,
	}
}
