package waiter

import (
	mathrand "math/rand"
	"strings"
	"testing"
	"time"

	"github.com/aws/smithy-go/rand"
)

func TestComputeDelay(t *testing.T) {
	cases := map[string]struct {
		totalAttempts       int64
		minDelay            time.Duration
		maxDelay            time.Duration
		maxWaitTime         time.Duration
		expectedMaxDelays   []time.Duration
		expectedError       string
		expectedMinAttempts int
	}{
		"standard": {
			totalAttempts:       8,
			minDelay:            2 * time.Second,
			maxDelay:            120 * time.Second,
			maxWaitTime:         300 * time.Second,
			expectedMaxDelays:   []time.Duration{2, 4, 8, 16, 32, 64, 120, 120},
			expectedMinAttempts: 8,
		},
		"zero minDelay": {
			totalAttempts: 3,
			minDelay:      0,
			maxDelay:      120 * time.Second,
			maxWaitTime:   300 * time.Second,
			expectedError: "minDelay must be greater than zero",
		},
		"zero maxDelay": {
			totalAttempts: 3,
			minDelay:      10 * time.Second,
			maxDelay:      0,
			maxWaitTime:   300 * time.Second,
			expectedError: "maxDelay must be greater than zero",
		},
		"zero remaining time": {
			totalAttempts:       3,
			minDelay:            10 * time.Second,
			maxDelay:            20 * time.Second,
			maxWaitTime:         0,
			expectedMaxDelays:   []time.Duration{0},
			expectedMinAttempts: 1,
		},
		"max wait time is less than min delay": {
			totalAttempts:       3,
			minDelay:            10 * time.Second,
			maxDelay:            20 * time.Second,
			maxWaitTime:         5 * time.Second,
			expectedMaxDelays:   []time.Duration{0},
			expectedMinAttempts: 1,
		},
		"large minDelay": {
			totalAttempts:       80,
			minDelay:            150 * time.Minute,
			maxDelay:            200 * time.Minute,
			maxWaitTime:         250 * time.Minute,
			expectedMinAttempts: 1,
		},
		"large maxDelay": {
			totalAttempts:       80,
			minDelay:            15 * time.Minute,
			maxDelay:            2000 * time.Minute,
			maxWaitTime:         250 * time.Minute,
			expectedMinAttempts: 5,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// mock smithy-go rand/#Reader
			r := rand.Reader
			defer func() {
				rand.Reader = r
			}()
			rand.Reader = mathrand.New(mathrand.NewSource(1))

			// mock waiter call
			delays, err := mockwait(c.totalAttempts, c.minDelay, c.maxDelay, c.maxWaitTime)

			if len(c.expectedError) != 0 {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				if e, a := c.expectedError, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expected error %v, got %v instead", e, a)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if e, a := c.expectedMinAttempts, len(delays); e > a {
				t.Logf("%v", delays)
				t.Fatalf("expected minimum attempts to be %v, got %v", e, a)
			}

			for i, expectedDelay := range c.expectedMaxDelays {
				if e, a := expectedDelay*time.Second, delays[i]; e < a {
					t.Fatalf("attempt %d : expected delay to be less than %v, got %v", i+1, e, a)
				}

				if e, a := c.minDelay, delays[i]; e > a && c.maxWaitTime > c.minDelay {
					t.Fatalf("attempt %d : expected delay to be more than %v, got %v", i+1, e, a)
				}
			}
			t.Logf("delays : %v", delays)
		})
	}
}

func mockwait(maxAttempts int64, minDelay, maxDelay, maxWaitTime time.Duration) ([]time.Duration, error) {
	delays := make([]time.Duration, 0)
	remainingTime := maxWaitTime
	var attempt int64

	for {
		attempt++

		if maxAttempts < attempt {
			break
		}

		delay, err := ComputeDelay(attempt, minDelay, maxDelay, remainingTime)
		if err != nil {
			return delays, err
		}

		delays = append(delays, delay)

		remainingTime -= delay
		if remainingTime < minDelay {
			break
		}
	}

	return delays, nil
}
