package http

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/smithy-go/middleware"
)

func ExampleResponse_deserializeMiddleware() {
	// Create the stack and provide the function that will create a new Request
	// when the SerializeStep is invoked.
	stack := middleware.NewStack("deserialize example", NewStackRequest)

	type Output struct {
		FooName  string
		BarCount int
	}

	// Add a Deserialize middleware that will extract the RawResponse and
	// deserialize into the target output type.
	stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("example deserialize",
		func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
			out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
		) {
			out, metadata, err = next.HandleDeserialize(ctx, in)
			if err != nil {
				return middleware.DeserializeOutput{}, metadata, err
			}

			metadata.Set("example-meta", "meta-value")

			rawResp := out.RawResponse.(*Response)
			out.Result = &Output{
				FooName: rawResp.Header.Get("foo-name"),
				BarCount: func() int {
					v, _ := strconv.Atoi(rawResp.Header.Get("bar-count"))
					return v
				}(),
			}

			return out, metadata, nil
		}),
		middleware.After,
	)

	// Mock example handler taking the request input and returning a response
	mockHandler := middleware.HandlerFunc(func(ctx context.Context, in interface{}) (
		output interface{}, metadata middleware.Metadata, err error,
	) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
		}
		resp.Header.Set("foo-name", "abc")
		resp.Header.Set("bar-count", "123")

		// The handler's returned response will be available as the
		// DeserializeOutput.RawResponse field.
		return &Response{
			Response: resp,
		}, metadata, nil
	})

	// Use the stack to decorate the handler then invoke the decorated handler
	// with the inputs.
	handler := middleware.DecorateHandler(mockHandler, stack)
	result, metadata, err := handler.Handle(context.Background(), struct{}{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to call operation, %v", err)
		return
	}

	// Cast the result returned by the handler to the expected Output type.
	res := result.(*Output)
	fmt.Println("FooName", res.FooName)
	fmt.Println("BarCount", res.BarCount)
	fmt.Println("Metadata:", "example-meta:", metadata.Get("example-meta"))

	// Output:
	// FooName abc
	// BarCount 123
	// Metadata: example-meta: meta-value
}
