package s3

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	defaultRetryMaxAttempts  = 3
	defaultRetryInitialDelay = 100 * time.Millisecond
	defaultRetryMaxDelay     = 1 * time.Second
)

type retryPolicy struct {
	maxAttempts  int
	initialDelay time.Duration
	maxDelay     time.Duration
	sleep        func(context.Context, time.Duration) error
}

func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		maxAttempts:  defaultRetryMaxAttempts,
		initialDelay: defaultRetryInitialDelay,
		maxDelay:     defaultRetryMaxDelay,
		sleep:        sleepWithContext,
	}
}

func (p retryPolicy) normalized() retryPolicy {
	if p.maxAttempts < 1 {
		p.maxAttempts = defaultRetryMaxAttempts
	}
	if p.initialDelay < 0 {
		p.initialDelay = 0
	}
	if p.maxDelay <= 0 {
		p.maxDelay = defaultRetryMaxDelay
	}
	if p.sleep == nil {
		p.sleep = sleepWithContext
	}
	return p
}

func retryAWS[T any](ctx context.Context, policy retryPolicy, fn func() (T, error)) (T, error) {
	var zero T
	p := policy.normalized()

	for attempt := 1; attempt <= p.maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		if attempt >= p.maxAttempts || !isRetryableAWSError(err) {
			return zero, err
		}
		if err := p.sleep(ctx, backoffForAttempt(p, attempt)); err != nil {
			return zero, err
		}
	}

	return zero, errors.New("retry exhausted")
}

func backoffForAttempt(policy retryPolicy, attempt int) time.Duration {
	delay := policy.initialDelay
	if attempt <= 1 || delay <= 0 {
		return delay
	}

	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= policy.maxDelay {
			return policy.maxDelay
		}
	}
	if delay > policy.maxDelay {
		return policy.maxDelay
	}
	return delay
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRetryableAWSError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := strings.ToLower(strings.TrimSpace(apiErr.ErrorCode()))
		switch code {
		case "throttling", "throttlingexception", "toomanyrequestsexception", "requestlimitexceeded", "slowdown", "requesttimeout", "requesttimeoutexception", "internalerror", "internalfailure", "serviceunavailable", "temporarilyunavailable", "priorrequestnotcomplete", "ec2throttledexception":
			return true
		}
	}

	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) {
		status := responseErr.HTTPStatusCode()
		if status == 429 || status == 500 || status == 502 || status == 503 || status == 504 {
			return true
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}
