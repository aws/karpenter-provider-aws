package middleware

import (
	"context"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"os"
	"testing"
)

func TestRecursionDetection(t *testing.T) {
	cases := map[string]struct {
		LambdaFuncName string
		TraceID        string
		HeaderBefore   string
		HeaderAfter    string
	}{
		"non lambda env and no trace ID header before": {},
		"with lambda env but no trace ID env variable, no trace ID header before": {
			LambdaFuncName: "some-function1",
		},
		"with lambda env and trace ID env variable, no trace ID header before": {
			LambdaFuncName: "some-function2",
			TraceID:        "traceID1",
			HeaderAfter:    "traceID1",
		},
		"with lambda env and trace ID env variable, has trace ID header before": {
			LambdaFuncName: "some-function3",
			TraceID:        "traceID2",
			HeaderBefore:   "traceID1",
			HeaderAfter:    "traceID1",
		},
		"with lambda env and trace ID (needs encoding) env variable, no trace ID header before": {
			LambdaFuncName: "some-function4",
			TraceID:        "traceID3\n",
			HeaderAfter:    "traceID3%0A",
		},
		"with lambda env and trace ID (contains chars must not be encoded) env variable, no trace ID header before": {
			LambdaFuncName: "some-function5",
			TraceID:        "traceID4-=;:+&[]{}\"'",
			HeaderAfter:    "traceID4-=;:+&[]{}\"'",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// clear current case's environment variables and restore them at the end of the test func goroutine
			restoreEnv := clearEnv()
			defer restoreEnv()

			setEnvVar(t, envAwsLambdaFunctionName, c.LambdaFuncName)
			setEnvVar(t, envAmznTraceID, c.TraceID)

			req := smithyhttp.NewStackRequest().(*smithyhttp.Request)
			if c.HeaderBefore != "" {
				req.Header.Set(amznTraceIDHeader, c.HeaderBefore)
			}
			var updatedRequest *smithyhttp.Request
			m := RecursionDetection{}
			_, _, err := m.HandleBuild(context.Background(),
				smithymiddleware.BuildInput{Request: req},
				smithymiddleware.BuildHandlerFunc(func(ctx context.Context, input smithymiddleware.BuildInput) (
					out smithymiddleware.BuildOutput, metadata smithymiddleware.Metadata, err error) {
					updatedRequest = input.Request.(*smithyhttp.Request)
					return out, metadata, nil
				}),
			)
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			if e, a := c.HeaderAfter, updatedRequest.Header.Get(amznTraceIDHeader); e != a {
				t.Errorf("expect header value %v found, got %v", e, a)
			}
		})
	}
}

// check if test case has environment variable and set to os if it has
func setEnvVar(t *testing.T, key, value string) {
	if value != "" {
		err := os.Setenv(key, value)
		if err != nil {
			t.Fatalf("expect no error, got %v", err)
		}
	}
}
