package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"

	"github.com/aws/aws-sdk-go-v2/aws/middleware"
	internalcontext "github.com/aws/aws-sdk-go-v2/internal/context"
	internalmiddleware "github.com/aws/aws-sdk-go-v2/internal/middleware"
	"github.com/aws/aws-sdk-go-v2/internal/sdk"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type options struct {
	HTTPClient httpClient
	RetryMode  aws.RetryMode
	Retryer    aws.Retryer
	Offset     *atomic.Int64
}

type MockClient struct {
	options options
}

func addRetry(stack *smithymiddleware.Stack, o options) error {
	attempt := retry.NewAttemptMiddleware(o.Retryer, smithyhttp.RequestCloner, func(m *retry.Attempt) {
		m.LogAttempts = false
	})
	return stack.Finalize.Add(attempt, smithymiddleware.After)
}

func addOffset(stack *smithymiddleware.Stack, o options) error {
	offsetMiddleware := internalmiddleware.AddTimeOffsetMiddleware{Offset: o.Offset}
	err := stack.Build.Add(&offsetMiddleware, smithymiddleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&offsetMiddleware, smithymiddleware.Before)
	if err != nil {
		return err
	}
	return nil
}

// Middleware to set a `Date` object that includes sdk time and offset
type MockAddDateHeader struct {
}

func (l *MockAddDateHeader) ID() string {
	return "MockAddDateHeader"
}

func (l *MockAddDateHeader) HandleFinalize(
	ctx context.Context, in smithymiddleware.FinalizeInput, next smithymiddleware.FinalizeHandler,
) (
	out smithymiddleware.FinalizeOutput, metadata smithymiddleware.Metadata, attemptError error,
) {
	req := in.Request.(*smithyhttp.Request)
	date := sdk.NowTime()
	skew := internalcontext.GetAttemptSkewContext(ctx)
	date = date.Add(skew)
	req.Header.Set("Date", date.Format(time.RFC850))
	return next.HandleFinalize(ctx, in)
}

// Middleware to deserialize the response which just says "OK" if the response is 200
type DeserializeFailIfNotHTTP200 struct {
}

func (*DeserializeFailIfNotHTTP200) ID() string {
	return "DeserializeFailIfNotHTTP200"
}

func (m *DeserializeFailIfNotHTTP200) HandleDeserialize(ctx context.Context, in smithymiddleware.DeserializeInput, next smithymiddleware.DeserializeHandler) (
	out smithymiddleware.DeserializeOutput, metadata smithymiddleware.Metadata, err error,
) {
	out, metadata, err = next.HandleDeserialize(ctx, in)
	if err != nil {
		return out, metadata, err
	}
	response, ok := out.RawResponse.(*smithyhttp.Response)
	if !ok {
		return out, metadata, fmt.Errorf("expected raw response to be set on testing")
	}
	if response.StatusCode != 200 {
		return out, metadata, mockRetryableError{true}
	}
	return out, metadata, err
}

func (c *MockClient) setupMiddleware(stack *smithymiddleware.Stack) error {
	err := error(nil)
	if c.options.Retryer != nil {
		err = addRetry(stack, c.options)
		if err != nil {
			return err
		}
	}
	if c.options.Offset != nil {
		err = addOffset(stack, c.options)
		if err != nil {
			return err
		}
	}
	err = stack.Finalize.Add(&MockAddDateHeader{}, smithymiddleware.After)
	if err != nil {
		return err
	}
	err = middleware.AddRecordResponseTiming(stack)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&DeserializeFailIfNotHTTP200{}, smithymiddleware.After)
	if err != nil {
		return err
	}
	return nil
}

func (c *MockClient) Do(ctx context.Context) (interface{}, error) {
	// setup middlewares
	ctx = smithymiddleware.ClearStackValues(ctx)
	stack := smithymiddleware.NewStack("stack", smithyhttp.NewStackRequest)
	err := c.setupMiddleware(stack)
	if err != nil {
		return nil, err
	}
	handler := smithymiddleware.DecorateHandler(smithyhttp.NewClientHandler(c.options.HTTPClient), stack)
	result, _, err := handler.Handle(ctx, 1)
	if err != nil {
		return nil, err
	}
	return result, err
}

type mockRetryableError struct{ b bool }

func (m mockRetryableError) RetryableError() bool { return m.b }
func (m mockRetryableError) Error() string {
	return fmt.Sprintf("mock retryable %t", m.b)
}

func failRequestIfSkewed() smithyhttp.ClientDoFunc {
	return func(req *http.Request) (*http.Response, error) {
		dateHeader := req.Header.Get("Date")
		if dateHeader == "" {
			return nil, fmt.Errorf("expected `Date` header to be set")
		}
		reqDate, err := time.Parse(time.RFC850, dateHeader)
		if err != nil {
			return nil, err
		}
		parsedReqTime := time.Now().Sub(reqDate)
		parsedReqTime = time.Duration.Abs(parsedReqTime)
		thresholdForSkewError := 4 * time.Minute
		if thresholdForSkewError-parsedReqTime <= 0 {
			return &http.Response{
				StatusCode: 403,
				Header: http.Header{
					"Date": {time.Now().Format(time.RFC850)},
				},
			}, nil
		}
		// else, return OK
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
		}, nil
	}
}

func TestSdkOffsetIsSet(t *testing.T) {
	nowTime := sdk.NowTime
	defer func() {
		sdk.NowTime = nowTime
	}()
	fiveMinuteSkew := func() time.Time {
		return time.Now().Add(5 * time.Minute)
	}
	sdk.NowTime = fiveMinuteSkew
	c := MockClient{
		options{
			HTTPClient: failRequestIfSkewed(),
		},
	}
	resp, err := c.Do(context.Background())
	if err == nil {
		t.Errorf("Expected first request to fail since clock skew logic has not run. Got %v and err %v", resp, err)
	}
}

func TestRetrySetsSkewInContext(t *testing.T) {
	defer resetDefaults(sdk.TestingUseNopSleep())
	fiveMinuteSkew := func() time.Time {
		return time.Now().Add(5 * time.Minute)
	}
	sdk.NowTime = fiveMinuteSkew
	c := MockClient{
		options{
			HTTPClient: failRequestIfSkewed(),
			Retryer: retry.NewStandard(func(s *retry.StandardOptions) {
			}),
		},
	}
	resp, err := c.Do(context.Background())
	if err != nil {
		t.Errorf("Expected request to succeed on retry. Got %v and err %v", resp, err)
	}
}

func TestSkewIsSetOnTheWholeClient(t *testing.T) {
	defer resetDefaults(sdk.TestingUseNopSleep())
	fiveMinuteSkew := func() time.Time {
		return time.Now().Add(5 * time.Minute)
	}
	sdk.NowTime = fiveMinuteSkew
	var offset atomic.Int64
	offset.Store(0)
	c := MockClient{
		options{
			HTTPClient: failRequestIfSkewed(),
			Retryer: retry.NewStandard(func(s *retry.StandardOptions) {
			}),
			Offset: &offset,
		},
	}
	resp, err := c.Do(context.Background())
	if err != nil {
		t.Errorf("Expected request to succeed on retry. Got %v and err %v", resp, err)
	}
	// Remove retryer so it has to succeed on first call
	c.options.Retryer = nil
	// same client, new request
	resp, err = c.Do(context.Background())
	if err != nil {
		t.Errorf("Expected second request to succeed since the skew should be set on the client. Got %v and err %v", resp, err)
	}
}

func resetDefaults(restoreSleepFunc func()) {
	sdk.NowTime = time.Now
	restoreSleepFunc()
}
