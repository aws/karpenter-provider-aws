package retry

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

var _ aws.Retryer = (*Standard)(nil)

func TestStandard_copyOptions(t *testing.T) {
	origDefaultRetryables := DefaultRetryables
	defer func() {
		DefaultRetryables = origDefaultRetryables
	}()
	DefaultRetryables = append([]IsErrorRetryable{}, DefaultRetryables...)

	origDefaultTimeouts := DefaultTimeouts
	defer func() {
		DefaultTimeouts = origDefaultTimeouts
	}()
	DefaultTimeouts = append([]IsErrorTimeout{}, DefaultTimeouts...)

	a := NewStandard(func(ao *StandardOptions) {
		ao.Retryables[0] = nil
		ao.Timeouts[0] = nil
	})

	if DefaultRetryables[0] == nil {
		t.Errorf("expect no change to global var")
	}

	if a.options.Retryables[0] != nil {
		t.Errorf("expect retryables to be changed")
	}

	if DefaultTimeouts[0] == nil {
		t.Errorf("expect no change to global var")
	}

	if a.options.Timeouts[0] != nil {
		t.Errorf("expect timeouts to be changed")
	}
}

func TestStandard_IsErrorRetryable(t *testing.T) {
	cases := map[string]struct {
		Retryable IsErrorRetryable
		Err       error
		Expect    bool
	}{
		"is retryable": {
			Expect: true,
			Err:    fmt.Errorf("expected error"),
			Retryable: IsErrorRetryableFunc(
				func(error) aws.Ternary {
					return aws.TrueTernary
				}),
		},
		"is not retryable": {
			Expect: false,
			Err:    fmt.Errorf("expected error"),
			Retryable: IsErrorRetryableFunc(
				func(error) aws.Ternary {
					return aws.FalseTernary
				}),
		},
		"unknown retryable": {
			Expect: false,
			Err:    fmt.Errorf("expected error"),
			Retryable: IsErrorRetryableFunc(
				func(error) aws.Ternary {
					return aws.UnknownTernary
				}),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewStandard(func(o *StandardOptions) {
				o.Retryables = []IsErrorRetryable{
					IsErrorRetryableFunc(
						func(err error) aws.Ternary {
							if e, a := c.Err, err; e != a {
								t.Fatalf("expect %v, error, got %v", e, a)
							}
							return c.Retryable.IsErrorRetryable(err)
						}),
				}
			})
			if e, a := c.Expect, r.IsErrorRetryable(c.Err); e != a {
				t.Errorf("expect %t retryable, got %t", e, a)
			}
		})
	}
}

func TestStandard_MaxAttempts(t *testing.T) {
	cases := map[string]struct {
		Max    int
		Expect int
	}{
		"defaults": {
			Expect: 3,
		},
		"custom": {
			Max:    10,
			Expect: 10,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewStandard(func(o *StandardOptions) {
				if c.Max != 0 {
					o.MaxAttempts = c.Max
				} else {
					c.Max = o.MaxAttempts
				}
			})
			if e, a := c.Max, r.MaxAttempts(); e != a {
				t.Errorf("expect %v max, got %v", e, a)
			}
		})
	}
}

func TestStandard_RetryDelay(t *testing.T) {
	cases := map[string]struct {
		Backoff     BackoffDelayer
		Attempt     int
		Err         error
		Assert      func(*testing.T, time.Duration, error)
		ExpectDelay time.Duration
		ExpectErr   error
	}{
		"success": {
			Attempt:     2,
			Err:         fmt.Errorf("expected error"),
			ExpectDelay: 10 * time.Millisecond,

			Backoff: BackoffDelayerFunc(
				func(attempt int, err error) (time.Duration, error) {
					return 10 * time.Millisecond, nil
				}),
		},
		"error": {
			Attempt:     2,
			Err:         fmt.Errorf("expected error"),
			ExpectDelay: 0,
			ExpectErr:   fmt.Errorf("failed get delay"),
			Backoff: BackoffDelayerFunc(
				func(attempt int, err error) (time.Duration, error) {
					return 0, fmt.Errorf("failed get delay")
				}),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewStandard(func(o *StandardOptions) {
				o.Backoff = BackoffDelayerFunc(
					func(attempt int, err error) (time.Duration, error) {
						if e, a := c.Err, err; e != a {
							t.Errorf("expect %v error, got %v", e, a)
						}
						if e, a := c.Attempt, attempt; e != a {
							t.Errorf("expect %v attempt, got %v", e, a)
						}
						return c.Backoff.BackoffDelay(attempt, err)
					})
			})

			delay, err := r.RetryDelay(c.Attempt, c.Err)
			if c.ExpectErr != nil {
				if e, a := c.ExpectErr.Error(), err.Error(); e != a {
					t.Errorf("expect %v error, got %v", e, a)
				}
			} else {
				if err != nil {
					t.Fatalf("expect no error, got %v", err)
				}
			}

			if e, a := c.ExpectDelay, delay; e != a {
				t.Errorf("expect %v delay, got %v", e, a)
			}
		})
	}
}

func TestStandard_retryEventuallySucceeds(t *testing.T) {
	ratelimiter := ratelimit.NewTokenRateLimit(500)
	retryer := NewStandard(func(o *StandardOptions) {
		o.MaxAttempts = 3
		o.RateLimiter = ratelimiter
		backoff := NewExponentialJitterBackoff(20 * time.Second)
		backoff.randFloat64 = func() (float64, error) {
			return 0.5, nil
		}
		o.Backoff = backoff
	})

	attempts := []struct {
		responseErr      error
		expectRetryable  bool
		expectRetryQuota uint
		expectDelay      time.Duration
	}{
		{
			responseErr:      newStubResponseError(500),
			expectRetryable:  true,
			expectRetryQuota: 495,
			expectDelay:      1 * time.Second,
		},
		{
			responseErr:      newStubResponseError(500),
			expectRetryable:  true,
			expectRetryQuota: 490,
			expectDelay:      2 * time.Second,
		},
		{
			expectRetryQuota: 496,
			// Refill 5 cost of retry + 1 successful response
		},
		{
			responseErr:      newStubResponseError(500),
			expectRetryable:  true,
			expectRetryQuota: 491,
			expectDelay:      8 * time.Second,
		},
		{
			responseErr:      newStubTimeoutError(),
			expectRetryable:  true,
			expectRetryQuota: 481,
			expectDelay:      20 * time.Second,
		},
		{
			expectRetryQuota: 492,
			// Refill 10 cost of retry + 1 successful response
		},
	}

	retryToken := nopRelease
	for i, attempt := range attempts {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			attemptToken, err := retryer.GetAttemptToken(context.Background())
			if err != nil {
				t.Fatalf("failed to get attempt token, %v", err)
			}

			if err := retryToken(attempt.responseErr); err != nil {
				t.Fatalf("expect release retry token not to fail, %v", err)
			}
			if err := attemptToken(attempt.responseErr); err != nil {
				t.Fatalf("expect release attempt token not to fail, %v", err)
			}

			if attempt.responseErr != nil {
				isRetryable := retryer.IsErrorRetryable(attempt.responseErr)
				if e, a := attempt.expectRetryable, isRetryable; e != a {
					t.Errorf("expect %v retryable, got %v", e, a)
				}

				retryToken = nopRelease
				if isRetryable {
					retryToken, err = retryer.GetRetryToken(context.Background(), attempt.responseErr)
					if err != nil {
						t.Fatalf("expect get retry token not to fail, %v", err)
					}
				}

				retryDelay, err := retryer.RetryDelay(i+1, attempt.responseErr)
				if err != nil {
					t.Fatalf("expect no retry delay error, got %v", err)
				}
				if e, a := attempt.expectDelay, retryDelay; e != a {
					t.Fatalf("expect %v retry delay, got %v", e, a)
				}
			}

			if e, a := attempt.expectRetryQuota, ratelimiter.Remaining(); e != a {
				t.Errorf("expect %v remaining tokens, got %v", e, a)
			}
		})
	}

}

func newStubResponseError(statusCode int) *smithyhttp.ResponseError {
	return &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{
			Response: &http.Response{
				StatusCode: statusCode,
				Header:     http.Header{},
				Body:       ioutil.NopCloser(bytes.NewReader(nil)),
			},
		},
	}
}

func newStubTimeoutError() *mockTimeoutErr {
	return &mockTimeoutErr{timeout: true}
}
