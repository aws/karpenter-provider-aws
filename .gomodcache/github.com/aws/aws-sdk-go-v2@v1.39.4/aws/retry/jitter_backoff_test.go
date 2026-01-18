package retry

import (
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/internal/timeconv"
)

func TestExponentialJitterBackoff_AttemptDelay(t *testing.T) {
	maxB := 1 - 1/float64(1<<53)

	cases := map[string]struct {
		MaxBackoff time.Duration
		RandFloat  func() (float64, error)
		Attempt    int
		Expect     time.Duration
	}{
		"min delay floor": {
			MaxBackoff: 20 * time.Second,
			RandFloat:  func() (float64, error) { return 0, nil },
			Attempt:    1,
			Expect:     0,
		},
		"min delay ceiling": {
			MaxBackoff: 20 * time.Second,
			RandFloat:  func() (float64, error) { return maxB, nil },
			Attempt:    1,
			Expect:     timeconv.FloatSecondsDur(maxB * float64(2)),
		},
		"attempt delay": {
			MaxBackoff: 20 * time.Second,
			RandFloat:  func() (float64, error) { return 0.5, nil },
			Attempt:    2,
			Expect:     timeconv.FloatSecondsDur(0.5 * float64(1<<2)),
		},
		"max delay": {
			MaxBackoff: 20 * time.Second,
			RandFloat:  func() (float64, error) { return maxB, nil },
			Attempt:    2147483647,
			Expect:     timeconv.FloatSecondsDur(maxB * math.Exp2(math.Log2(20))),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			j := NewExponentialJitterBackoff(c.MaxBackoff)
			j.randFloat64 = c.RandFloat

			d, err := j.BackoffDelay(c.Attempt, nil)
			if err != nil {
				t.Fatalf("expect not error, %v", err)
			}

			if e, a := c.Expect, d; e != a {
				t.Errorf("expect %v delay, got %v", e, a)
			}
		})
	}
}
