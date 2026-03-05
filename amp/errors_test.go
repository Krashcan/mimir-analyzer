package amp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestAMPError_ImplementsErrorInterface(t *testing.T) {
	var err error = &AMPError{
		Category: CategoryAuth,
		Message:  "test error",
	}
	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error")
	}
}

func TestAMPError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	ampErr := &AMPError{
		Category: CategoryServerError,
		Message:  "wrapped",
		Cause:    cause,
	}
	if !errors.Is(ampErr, cause) {
		t.Error("Unwrap should return the cause so errors.Is works")
	}
}

func TestAMPError_ErrorMessage(t *testing.T) {
	ampErr := &AMPError{
		Category:   CategoryAuth,
		Message:    "credentials expired",
		StatusCode: 403,
	}
	got := ampErr.Error()
	if got != "credentials expired" {
		t.Errorf("Error() = %q, want %q", got, "credentials expired")
	}
}

func TestClassifyError_DNSFailure(t *testing.T) {
	dnsErr := &net.DNSError{
		Err:  "no such host",
		Name: "bad-endpoint.example.com",
	}
	ampErr := ClassifyError(dnsErr)
	if ampErr.Category != CategoryConnectivity {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryConnectivity)
	}
	if ampErr.Cause != dnsErr {
		t.Error("Cause should be the original DNS error")
	}
}

func TestClassifyError_ConnectionRefused(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: fmt.Errorf("connection refused"),
	}
	ampErr := ClassifyError(opErr)
	if ampErr.Category != CategoryConnectivity {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryConnectivity)
	}
}

func TestClassifyError_Timeout(t *testing.T) {
	ampErr := ClassifyError(context.DeadlineExceeded)
	if ampErr.Category != CategoryTimeout {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryTimeout)
	}
}

func TestClassifyError_AlreadyAMPError(t *testing.T) {
	original := &AMPError{
		Category:   CategoryAuth,
		Message:    "already classified",
		StatusCode: 401,
	}
	result := ClassifyError(original)
	if result != original {
		t.Error("ClassifyError should return the same *AMPError if already classified")
	}
}

func TestClassifyError_UnknownError(t *testing.T) {
	unknown := fmt.Errorf("something weird happened")
	ampErr := ClassifyError(unknown)
	if ampErr.Category != CategoryServerError {
		t.Errorf("category = %q, want %q", ampErr.Category, CategoryServerError)
	}
	if ampErr.Cause != unknown {
		t.Error("Cause should be the original error")
	}
}
