package s3

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/smithy-go"
)

type testRetryAPIError struct {
	code string
	msg  string
}

func (e testRetryAPIError) ErrorCode() string             { return e.code }
func (e testRetryAPIError) ErrorMessage() string          { return e.msg }
func (e testRetryAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultServer }
func (e testRetryAPIError) Error() string                 { return e.code + ": " + e.msg }

type testTimeoutError struct{}

func (e testTimeoutError) Error() string   { return "timeout" }
func (e testTimeoutError) Timeout() bool   { return true }
func (e testTimeoutError) Temporary() bool { return true }

func TestIsRetryableAWSError(t *testing.T) {
	if !isRetryableAWSError(testRetryAPIError{code: "ThrottlingException", msg: "slow down"}) {
		t.Fatal("expected throttling error to be retryable")
	}
	if !isRetryableAWSError(testTimeoutError{}) {
		t.Fatal("expected timeout network error to be retryable")
	}
	if isRetryableAWSError(context.Canceled) {
		t.Fatal("expected context cancellation to be non-retryable")
	}
	if isRetryableAWSError(testRetryAPIError{code: "AccessDenied", msg: "forbidden"}) {
		t.Fatal("expected AccessDenied to be non-retryable")
	}
}

func TestRetryAWS_RetriesThenSucceeds(t *testing.T) {
	attempts := 0
	policy := retryPolicy{
		maxAttempts:  3,
		initialDelay: time.Millisecond,
		maxDelay:     2 * time.Millisecond,
		sleep:        func(context.Context, time.Duration) error { return nil },
	}

	value, err := retryAWS(context.Background(), policy, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, testRetryAPIError{code: "Throttling", msg: "slow down"}
		}
		return 7, nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if value != 7 {
		t.Fatalf("expected value 7, got %d", value)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryAWS_DoesNotRetryNonRetryable(t *testing.T) {
	attempts := 0
	_, err := retryAWS(context.Background(), retryPolicy{maxAttempts: 3, sleep: func(context.Context, time.Duration) error { return nil }}, func() (int, error) {
		attempts++
		return 0, errors.New("validation failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}
