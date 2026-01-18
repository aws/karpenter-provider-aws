/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authentication

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
)

var _ = Describe("Authentication Webhooks", func() {

	const (
		gvkJSONv1 = `"kind":"TokenReview","apiVersion":"authentication.k8s.io/v1"`
	)

	Describe("HTTP Handler", func() {
		var respRecorder *httptest.ResponseRecorder
		webhook := &Webhook{
			Handler: nil,
		}
		BeforeEach(func() {
			respRecorder = &httptest.ResponseRecorder{
				Body: bytes.NewBuffer(nil),
			}
		})

		It("should return bad-request when given an empty body", func() {
			req := &http.Request{Body: nil}

			expected := `{"metadata":{},"spec":{},"status":{"user":{},"error":"request body is empty"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(BeComparableTo(expected))
		})

		It("should return bad-request when given the wrong content-type", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/foo"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBuffer(nil)},
			}

			expected := `{"metadata":{},"spec":{},"status":{"user":{},"error":"contentType=application/foo, expected application/json"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return bad-request when given an undecodable body", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString("{")},
			}

			expected := `{"metadata":{},"spec":{},"status":{"user":{},"error":"couldn't get version/kind; json parse error: unexpected end of JSON input"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return bad-request when given an undecodable body", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"token":""}}`)},
			}

			expected := `{"metadata":{},"spec":{},"status":{"user":{},"error":"token is empty"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should error when given a NoBody", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   http.NoBody,
			}

			expected := `{"metadata":{},"spec":{},"status":{"user":{},"error":"request body is empty"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should error when given an infinite body", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: rand.Reader},
			}

			expected := `{"metadata":{},"spec":{},"status":{"user":{},"error":"request entity is too large; limit is 1048576 bytes"}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the response given by the handler with version defaulted to v1", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"token":"foobar"}}`)},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{},"spec":{},"status":{"authenticated":true,"user":{}}}
`, gvkJSONv1)

			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the v1 response given by the handler", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(fmt.Sprintf(`{%s,"spec":{"token":"foobar"}}`, gvkJSONv1))},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{},"spec":{},"status":{"authenticated":true,"user":{}}}
`, gvkJSONv1)
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should present the Context from the HTTP request, if any", func(specContext SpecContext) {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"token":"foobar"}}`)},
			}
			type ctxkey int
			const key ctxkey = 1
			const value = "from-ctx"
			webhook := &Webhook{
				Handler: &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						<-ctx.Done()
						return Authenticated(ctx.Value(key).(string), authenticationv1.UserInfo{})
					},
				},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{},"spec":{},"status":{"authenticated":true,"user":{},"error":%q}}
`, gvkJSONv1, value)

			ctx, cancel := context.WithCancel(context.WithValue(specContext, key, value))
			cancel()
			webhook.ServeHTTP(respRecorder, req.WithContext(ctx))
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should mutate the Context from the HTTP request, if func supplied", func(specContext SpecContext) {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Method: http.MethodPost,
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"spec":{"token":"foobar"}}`)},
			}
			type ctxkey int
			const key ctxkey = 1
			webhook := &Webhook{
				Handler: &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						return Authenticated(ctx.Value(key).(string), authenticationv1.UserInfo{})
					},
				},
				WithContextFunc: func(ctx context.Context, r *http.Request) context.Context {
					return context.WithValue(ctx, key, r.Header["Content-Type"][0])
				},
			}

			expected := fmt.Sprintf(`{%s,"metadata":{},"spec":{},"status":{"authenticated":true,"user":{},"error":%q}}
`, gvkJSONv1, "application/json")

			ctx, cancel := context.WithCancel(specContext)
			cancel()
			webhook.ServeHTTP(respRecorder, req.WithContext(ctx))
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})
	})
})

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type fakeHandler struct {
	invoked bool
	fn      func(context.Context, Request) Response
}

func (h *fakeHandler) Handle(ctx context.Context, req Request) Response {
	h.invoked = true
	if h.fn != nil {
		return h.fn(ctx, req)
	}
	return Response{TokenReview: authenticationv1.TokenReview{
		Status: authenticationv1.TokenReviewStatus{
			Authenticated: true,
		},
	}}
}
