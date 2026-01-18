package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/smithy-go"
	"github.com/aws/smithy-go/middleware"
)

func ExampleResponseError() {
	stack := middleware.NewStack("my cool stack", NewStackRequest)

	stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("wrap http response error",
		func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
			out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
		) {
			out, metadata, err = next.HandleDeserialize(ctx, in)
			if err == nil {
				// Nothing to do when there is no error.
				return out, metadata, err
			}

			rawResp, ok := out.RawResponse.(*Response)
			if !ok {
				// No raw response to wrap with.
				return out, metadata, err
			}

			// Wrap the returned error with the response error containing the
			// returned response.
			err = &ResponseError{
				Response: rawResp,
				Err:      err,
			}

			return out, metadata, err
		}),
		middleware.After,
	)

	stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("deserialize error",
		func(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (
			out middleware.DeserializeOutput, metadata middleware.Metadata, err error,
		) {
			out, metadata, err = next.HandleDeserialize(ctx, in)
			if err != nil {
				return middleware.DeserializeOutput{}, metadata, err
			}

			rawResp := out.RawResponse.(*Response)
			if rawResp.StatusCode == 200 {
				return out, metadata, nil
			}

			// Deserialize the response API error values.
			err = &smithy.GenericAPIError{
				Code:    rawResp.Header.Get("Error-Code"),
				Message: rawResp.Header.Get("Error-Message"),
			}

			return out, metadata, err
		}),
		middleware.After,
	)

	// Mock example handler taking the request input and returning a response
	mockHandler := middleware.HandlerFunc(func(ctx context.Context, in interface{}) (
		output interface{}, metadata middleware.Metadata, err error,
	) {
		// populate the mock response with an API error and additional data.
		resp := &http.Response{
			StatusCode: 404,
			Header: http.Header{
				"Extra-Header":  []string{"foo value"},
				"Error-Code":    []string{"FooException"},
				"Error-Message": []string{"some message about the error"},
			},
		}

		// The handler's returned response will be available as the
		// DeserializeOutput.RawResponse field.
		return &Response{
			Response: resp,
		}, metadata, nil
	})

	// Use the stack to decorate the handler then invoke the decorated handler
	// with the inputs.
	handler := middleware.DecorateHandler(mockHandler, stack)
	_, _, err := handler.Handle(context.Background(), struct{}{})
	if err == nil {
		fmt.Printf("expect error, got none")
		return
	}

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			fmt.Printf("request failed: %s, %s\n", apiErr.ErrorCode(), apiErr.ErrorMessage())
		}

		var respErr *ResponseError
		if errors.As(err, &respErr) {
			fmt.Printf("response status: %v\n", respErr.HTTPStatusCode())
			fmt.Printf("response header: %v\n", respErr.HTTPResponse().Header.Get("Extra-Header"))
		}
	}

	// Output:
	// request failed: FooException, some message about the error
	// response status: 404
	// response header: foo value
}
