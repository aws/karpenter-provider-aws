package retry

import (
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type mockTemporaryError struct{ b bool }

func (m mockTemporaryError) Temporary() bool { return m.b }
func (m mockTemporaryError) Error() string {
	return fmt.Sprintf("mock temporary %t", m.b)
}

type mockTimeoutError struct{ b bool }

func (m mockTimeoutError) Timeout() bool { return m.b }
func (m mockTimeoutError) Error() string {
	return fmt.Sprintf("mock timeout %t", m.b)
}

type mockRetryableError struct{ b bool }

func (m mockRetryableError) RetryableError() bool { return m.b }
func (m mockRetryableError) Error() string {
	return fmt.Sprintf("mock retryable %t", m.b)
}

type mockCanceledError struct{ b bool }

func (m mockCanceledError) CanceledError() bool { return m.b }
func (m mockCanceledError) Error() string {
	return fmt.Sprintf("mock canceled %t", m.b)
}

type mockStatusCodeError struct{ code int }

func (m mockStatusCodeError) HTTPStatusCode() int { return m.code }
func (m mockStatusCodeError) Error() string {
	return fmt.Sprintf("status code error, %v", m.code)
}

type mockConnectionError struct{ err error }

func (m *mockConnectionError) ConnectionError() bool {
	return true
}
func (m *mockConnectionError) Error() string {
	return fmt.Sprintf("request error: %v", m.err)
}
func (m *mockConnectionError) Unwrap() error {
	return m.err
}

type mockErrorCodeError struct {
	code string
	err  error
}

func (m *mockErrorCodeError) ErrorCode() string { return m.code }
func (m *mockErrorCodeError) Error() string {
	return fmt.Sprintf("%v: mock error", m.code)
}
func (m *mockErrorCodeError) Unwrap() error {
	return m.err
}

func TestRetryConnectionErrors(t *testing.T) {
	cases := map[string]struct {
		Err       error
		Retryable aws.Ternary
	}{
		"nested connection reset": {
			Retryable: aws.TrueTernary,
			Err: fmt.Errorf("serialization error, %w",
				fmt.Errorf("connection reset")),
		},
		"top level connection reset": {
			Retryable: aws.TrueTernary,
			Err:       fmt.Errorf("connection reset"),
		},
		"wrapped connection reset": {
			Retryable: aws.TrueTernary,
			Err:       fmt.Errorf("some error: %w", fmt.Errorf("connection reset")),
		},
		"url.Error connection refused": {
			Retryable: aws.TrueTernary,
			Err: fmt.Errorf("some error, %w", &url.Error{
				Err: fmt.Errorf("connection refused"),
			}),
		},
		"other connection refused": {
			Retryable: aws.UnknownTernary,
			Err:       fmt.Errorf("connection refused"),
		},
		"nil error connection reset": {
			Retryable: aws.UnknownTernary,
		},
		"some other error": {
			Retryable: aws.UnknownTernary,
			Err:       fmt.Errorf("some error: %w", fmt.Errorf("something bad")),
		},
		"request send error": {
			Retryable: aws.TrueTernary,
			Err: fmt.Errorf("some error: %w", &mockConnectionError{err: &url.Error{
				Err: fmt.Errorf("another error"),
			}}),
		},
		"temporary error": {
			Retryable: aws.TrueTernary,
			Err:       &mockErrorCodeError{code: "SomeCode", err: mockTemporaryError{b: true}},
		},
		"timeout error": {
			Retryable: aws.TrueTernary,
			Err:       fmt.Errorf("some error: %w", mockTimeoutError{b: true}),
		},
		"timeout false error": {
			Retryable: aws.UnknownTernary,
			Err:       fmt.Errorf("some error: %w", mockTimeoutError{b: false}),
		},
		"net.OpError dial": {
			Retryable: aws.TrueTernary,
			Err: &net.OpError{
				Op:  "dial",
				Err: mockTimeoutError{b: false},
			},
		},
		"net.OpError nested": {
			Retryable: aws.TrueTernary,
			Err: &net.OpError{
				Op:  "read",
				Err: fmt.Errorf("some error %w", mockTimeoutError{b: true}),
			},
		},
		"net.ErrClosed": {
			Retryable: aws.TrueTernary,
			Err:       net.ErrClosed,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var r RetryableConnectionError

			retryable := r.IsErrorRetryable(c.Err)
			if e, a := c.Retryable, retryable; e != a {
				t.Errorf("expect %v retryable, got %v", e, a)
			}
		})
	}
}

func TestRetryHTTPStatusCodes(t *testing.T) {
	cases := map[string]struct {
		Err    error
		Expect aws.Ternary
	}{
		"top level": {
			Err:    &mockStatusCodeError{code: 500},
			Expect: aws.TrueTernary,
		},
		"nested": {
			Err:    fmt.Errorf("some error, %w", &mockStatusCodeError{code: 500}),
			Expect: aws.TrueTernary,
		},
		"response error": {
			Err: fmt.Errorf("some error, %w", &mockErrorCodeError{
				code: "SomeCode",
				err:  &mockStatusCodeError{code: 502},
			}),
			Expect: aws.TrueTernary,
		},
	}

	r := RetryableHTTPStatusCode{Codes: map[int]struct{}{
		500: {},
		502: {},
	}}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if e, a := c.Expect, r.IsErrorRetryable(c.Err); e != a {
				t.Errorf("expect %v, got %v", e, a)
			}
		})
	}
}

func TestRetryErrorCodes(t *testing.T) {
	cases := map[string]struct {
		Err    error
		Expect aws.Ternary
	}{
		"retryable code": {
			Err: &MaxAttemptsError{
				Err: &mockErrorCodeError{code: "ErrorCode1"},
			},
			Expect: aws.TrueTernary,
		},
		"not retryable code": {
			Err: &MaxAttemptsError{
				Err: &mockErrorCodeError{code: "SomeErroCode"},
			},
			Expect: aws.UnknownTernary,
		},
		"other error": {
			Err:    fmt.Errorf("some other error"),
			Expect: aws.UnknownTernary,
		},
	}

	r := RetryableErrorCode{Codes: map[string]struct{}{
		"ErrorCode1": {},
		"ErrorCode2": {},
	}}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if e, a := c.Expect, r.IsErrorRetryable(c.Err); e != a {
				t.Errorf("expect %v, got %v", e, a)
			}
		})
	}
}

func TestCanceledError(t *testing.T) {
	cases := map[string]struct {
		Err    error
		Expect aws.Ternary
	}{
		"canceled error": {
			Err: fmt.Errorf("some error, %w", &aws.RequestCanceledError{
				Err: fmt.Errorf(":("),
			}),
			Expect: aws.FalseTernary,
		},
		"canceled retryable error": {
			Err: fmt.Errorf("some error, %w", &aws.RequestCanceledError{
				Err: mockRetryableError{b: true},
			}),
			Expect: aws.FalseTernary,
		},
		"not canceled error": {
			Err:    fmt.Errorf("some error, %w", mockCanceledError{b: false}),
			Expect: aws.UnknownTernary,
		},
		"retryable error": {
			Err:    fmt.Errorf("some error, %w", mockRetryableError{b: true}),
			Expect: aws.TrueTernary,
		},
	}

	r := IsErrorRetryables{
		NoRetryCanceledError{},
		RetryableError{},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if e, a := c.Expect, r.IsErrorRetryable(c.Err); e != a {
				t.Errorf("Expect %v retryable, got %v", e, a)
			}
		})
	}
}

func TestDNSError(t *testing.T) {
	cases := map[string]struct {
		Err    error
		Expect aws.Ternary
	}{
		"IsNotFound": {
			Err: &net.DNSError{
				IsNotFound: true,
			},
			Expect: aws.FalseTernary,
		},
		"Temporary (IsTimeout)": {
			Err: &net.DNSError{
				IsTimeout: true,
			},
			Expect: aws.TrueTernary,
		},
		"Temporary (IsTemporary)": {
			Err: &net.DNSError{
				IsTemporary: true,
			},
			Expect: aws.TrueTernary,
		},
		"Temporary() == false but it falls through": {
			Err: &net.OpError{
				Op:  "dial",
				Err: &net.DNSError{},
			},
			Expect: aws.TrueTernary,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var r RetryableConnectionError

			retryable := r.IsErrorRetryable(c.Err)
			if e, a := c.Expect, retryable; e != a {
				t.Errorf("expect %v retryable, got %v", e, a)
			}
		})
	}
}
