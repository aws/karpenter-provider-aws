/*
Copyright 2018 The Kubernetes Authors.

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

package admission

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
)

var _ = Describe("Admission Webhooks", func() {

	const (
		gvkJSONv1      = `"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1"`
		gvkJSONv1beta1 = `"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1beta1"`
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

			expected := `{"response":{"uid":"","allowed":false,"status":{"metadata":{},"message":"request body is empty","code":400}}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return bad-request when given the wrong content-type", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/foo"}},
				Body:   nopCloser{Reader: bytes.NewBuffer(nil)},
			}

			expected :=
				`{"response":{"uid":"","allowed":false,"status":{"metadata":{},"message":"contentType=application/foo, expected application/json","code":400}}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return bad-request when given an undecodable body", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString("{")},
			}

			expected :=
				`{"response":{"uid":"","allowed":false,"status":{"metadata":{},"message":"couldn't get version/kind; json parse error: unexpected end of JSON input","code":400}}}
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

			expected := `{"response":{"uid":"","allowed":false,"status":{"metadata":{},"message":"request body is empty","code":400}}}
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

			expected := `{"response":{"uid":"","allowed":false,"status":{"metadata":{},"message":"request entity is too large; limit is 7340032 bytes","code":413}}}
`
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the response given by the handler with version defaulted to v1", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"request":{}}`)},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"response":{"uid":"","allowed":true,"status":{"metadata":{},"code":200}}}
`, gvkJSONv1)
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the v1 response given by the handler", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(fmt.Sprintf(`{%s,"request":{}}`, gvkJSONv1))},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"response":{"uid":"","allowed":true,"status":{"metadata":{},"code":200}}}
`, gvkJSONv1)
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should return the v1beta1 response given by the handler", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(fmt.Sprintf(`{%s,"request":{}}`, gvkJSONv1beta1))},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			expected := fmt.Sprintf(`{%s,"response":{"uid":"","allowed":true,"status":{"metadata":{},"code":200}}}
`, gvkJSONv1beta1)
			webhook.ServeHTTP(respRecorder, req)
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should present the Context from the HTTP request, if any", func(specCtx SpecContext) {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"request":{}}`)},
			}
			type ctxkey int
			const key ctxkey = 1
			const value = "from-ctx"
			webhook := &Webhook{
				Handler: &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						<-ctx.Done()
						return Allowed(ctx.Value(key).(string))
					},
				},
			}

			expected := fmt.Sprintf(`{%s,"response":{"uid":"","allowed":true,"status":{"metadata":{},"message":%q,"code":200}}}
`, gvkJSONv1, value)

			ctx, cancel := context.WithCancel(context.WithValue(specCtx, key, value))
			cancel()
			webhook.ServeHTTP(respRecorder, req.WithContext(ctx))
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should mutate the Context from the HTTP request, if func supplied", func(specCtx SpecContext) {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(`{"request":{}}`)},
			}
			type ctxkey int
			const key ctxkey = 1
			webhook := &Webhook{
				Handler: &fakeHandler{
					fn: func(ctx context.Context, req Request) Response {
						return Allowed(ctx.Value(key).(string))
					},
				},
				WithContextFunc: func(ctx context.Context, r *http.Request) context.Context {
					return context.WithValue(ctx, key, r.Header["Content-Type"][0])
				},
			}

			expected := fmt.Sprintf(`{%s,"response":{"uid":"","allowed":true,"status":{"metadata":{},"message":%q,"code":200}}}
`, gvkJSONv1, "application/json")

			ctx, cancel := context.WithCancel(specCtx)
			cancel()
			webhook.ServeHTTP(respRecorder, req.WithContext(ctx))
			Expect(respRecorder.Body.String()).To(Equal(expected))
		})

		It("should never run into circular calling if the writer has broken", func() {
			req := &http.Request{
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   nopCloser{Reader: bytes.NewBufferString(fmt.Sprintf(`{%s,"request":{}}`, gvkJSONv1))},
			}
			webhook := &Webhook{
				Handler: &fakeHandler{},
			}

			bw := &brokenWriter{ResponseWriter: respRecorder}
			Eventually(func() int {
				// This should not be blocked by the circular calling of writeResponse and writeAdmissionResponse
				webhook.ServeHTTP(bw, req)
				return respRecorder.Body.Len()
			}, time.Second*3).Should(Equal(0))
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
	return Response{AdmissionResponse: admissionv1.AdmissionResponse{
		Allowed: true,
	}}
}

type brokenWriter struct {
	http.ResponseWriter
}

func (bw *brokenWriter) Write(buf []byte) (int, error) {
	return 0, fmt.Errorf("mock: write: broken pipe")
}
