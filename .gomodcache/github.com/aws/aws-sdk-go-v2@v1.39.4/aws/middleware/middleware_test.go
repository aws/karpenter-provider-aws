package middleware_test

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/internal/rand"
	"github.com/aws/aws-sdk-go-v2/internal/sdk"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestClientRequestID(t *testing.T) {
	oReader := rand.Reader
	defer func() {
		rand.Reader = oReader
	}()
	rand.Reader = bytes.NewReader(make([]byte, 16))

	mid := middleware.ClientRequestID{}

	in := smithymiddleware.BuildInput{Request: &smithyhttp.Request{Request: &http.Request{Header: make(http.Header)}}}
	ctx := context.Background()
	_, _, err := mid.HandleBuild(ctx, in, smithymiddleware.BuildHandlerFunc(func(ctx context.Context, input smithymiddleware.BuildInput) (
		out smithymiddleware.BuildOutput, metadata smithymiddleware.Metadata, err error,
	) {
		req := in.Request.(*smithyhttp.Request)

		value := req.Header.Get("amz-sdk-invocation-id")

		expected := "00000000-0000-4000-8000-000000000000"
		if value != expected {
			t.Errorf("expect %v, got %v", expected, value)
		}

		return out, metadata, err
	}))
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	in = smithymiddleware.BuildInput{}
	_, _, err = mid.HandleBuild(ctx, in, nil)
	if err != nil {
		if e, a := "unknown transport type", err.Error(); !strings.Contains(a, e) {
			t.Errorf("expected %q, got %q", e, a)
		}
	} else {
		t.Errorf("expected error, got %q", err)
	}
}

func TestAttemptClockSkewHandler(t *testing.T) {
	cases := map[string]struct {
		Next              smithymiddleware.DeserializeHandlerFunc
		ResponseAt        func() time.Time
		ExpectAttemptSkew time.Duration
		ExpectServerTime  time.Time
		ExpectResponseAt  time.Time
	}{
		"no response": {
			Next: func(ctx context.Context, in smithymiddleware.DeserializeInput,
			) (out smithymiddleware.DeserializeOutput, m smithymiddleware.Metadata, err error) {
				return out, m, err
			},
			ResponseAt: func() time.Time {
				return time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
			},
			ExpectResponseAt: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		"failed response": {
			Next: func(ctx context.Context, in smithymiddleware.DeserializeInput,
			) (out smithymiddleware.DeserializeOutput, m smithymiddleware.Metadata, err error) {
				out.RawResponse = &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 0,
						Header:     http.Header{},
					},
				}
				return out, m, err
			},
			ResponseAt: func() time.Time {
				return time.Date(2020, 6, 7, 8, 9, 10, 0, time.UTC)
			},
			ExpectResponseAt: time.Date(2020, 6, 7, 8, 9, 10, 0, time.UTC),
		},
		"no date header response": {
			Next: func(ctx context.Context, in smithymiddleware.DeserializeInput,
			) (out smithymiddleware.DeserializeOutput, m smithymiddleware.Metadata, err error) {
				out.RawResponse = &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 200,
						Header:     http.Header{},
					},
				}
				return out, m, err
			},
			ResponseAt: func() time.Time {
				return time.Date(2020, 11, 12, 13, 14, 15, 0, time.UTC)
			},
			ExpectResponseAt: time.Date(2020, 11, 12, 13, 14, 15, 0, time.UTC),
		},
		"invalid date header response": {
			Next: func(ctx context.Context, in smithymiddleware.DeserializeInput,
			) (out smithymiddleware.DeserializeOutput, m smithymiddleware.Metadata, err error) {
				out.RawResponse = &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 200,
						Header: http.Header{
							"Date": []string{"abc123"},
						},
					},
				}
				return out, m, err
			},
			ResponseAt: func() time.Time {
				return time.Date(2020, 1, 2, 16, 17, 18, 0, time.UTC)
			},
			ExpectResponseAt: time.Date(2020, 1, 2, 16, 17, 18, 0, time.UTC),
		},
		"date response": {
			Next: func(ctx context.Context, in smithymiddleware.DeserializeInput,
			) (out smithymiddleware.DeserializeOutput, m smithymiddleware.Metadata, err error) {
				out.RawResponse = &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: 200,
						Header: http.Header{
							"Date": []string{"Thu, 05 Mar 2020 22:25:15 GMT"},
						},
					},
				}
				return out, m, err
			},
			ResponseAt: func() time.Time {
				return time.Date(2020, 3, 5, 22, 25, 17, 0, time.UTC)
			},
			ExpectResponseAt:  time.Date(2020, 3, 5, 22, 25, 17, 0, time.UTC),
			ExpectServerTime:  time.Date(2020, 3, 5, 22, 25, 15, 0, time.UTC),
			ExpectAttemptSkew: -2 * time.Second,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if c.ResponseAt != nil {
				sdkTime := sdk.NowTime
				defer func() {
					sdk.NowTime = sdkTime
				}()
				sdk.NowTime = c.ResponseAt
			}
			mw := middleware.RecordResponseTiming{}
			_, metadata, err := mw.HandleDeserialize(context.Background(), smithymiddleware.DeserializeInput{}, c.Next)
			if err != nil {
				t.Errorf("expect no error, got %v", err)
			}

			if v, ok := middleware.GetResponseAt(metadata); ok {
				if !reflect.DeepEqual(v, c.ExpectResponseAt) {
					t.Fatalf("expected %v, got %v", c.ExpectResponseAt, v)
				}
			} else if !c.ExpectResponseAt.IsZero() {
				t.Fatal("expected response at to be set in metadata, was not")
			}

			if v, ok := middleware.GetServerTime(metadata); ok {
				if !reflect.DeepEqual(v, c.ExpectServerTime) {
					t.Fatalf("expected %v, got %v", c.ExpectServerTime, v)
				}
			} else if !c.ExpectServerTime.IsZero() {
				t.Fatal("expected server time to be set in metadata, was not")
			}

			if v, ok := middleware.GetAttemptSkew(metadata); ok {
				if !reflect.DeepEqual(v, c.ExpectAttemptSkew) {
					t.Fatalf("expected %v, got %v", c.ExpectAttemptSkew, v)
				}
			} else if c.ExpectAttemptSkew != 0 {
				t.Fatal("expected attempt skew to be set in metadata, was not")
			}
		})
	}
}
