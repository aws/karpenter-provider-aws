package retry

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type mockTimeoutErr struct{ timeout bool }

func (m mockTimeoutErr) Timeout() bool { return m.timeout }
func (m mockTimeoutErr) Error() string {
	return fmt.Sprintf("mock timeout, %t", m.timeout)
}

func TestIsTimeoutError(t *testing.T) {
	cases := map[string]struct {
		Err    error
		Check  IsErrorTimeout
		Expect aws.Ternary
	}{
		"TimeouterError true": {
			Err:    fmt.Errorf("nested error, %w", mockTimeoutErr{timeout: true}),
			Check:  TimeouterError{},
			Expect: aws.TrueTernary,
		},
		"TimeouterError false": {
			Err:    fmt.Errorf("nested error, %w", mockTimeoutErr{timeout: false}),
			Check:  TimeouterError{},
			Expect: aws.FalseTernary,
		},
		"other error": {
			Err:   fmt.Errorf("some other error"),
			Check: TimeouterError{},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if e, a := c.Expect, c.Check.IsErrorTimeout(c.Err); e != a {
				t.Errorf("expect %v, got %v", e, a)
			}
		})
	}
}
