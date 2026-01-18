/*
Copyright 2014 The Kubernetes Authors.

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

package healthz_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

const (
	contentType = "text/plain; charset=utf-8"
)

func requestTo(handler http.Handler, dest string) *httptest.ResponseRecorder {
	req, err := http.NewRequest("GET", dest, nil)
	Expect(err).NotTo(HaveOccurred())
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	return resp
}

var _ = Describe("Healthz Handler", func() {
	Describe("the aggregated endpoint", func() {
		It("should return healthy if all checks succeed", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"ok1": healthz.Ping,
				"ok2": healthz.Ping,
			}}

			resp := requestTo(handler, "/")
			Expect(resp.Code).To(Equal(http.StatusOK))
		})

		It("should return unhealthy if at least one check fails", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"ok1": healthz.Ping,
				"bad1": func(req *http.Request) error {
					return errors.New("blech")
				},
			}}

			resp := requestTo(handler, "/")
			Expect(resp.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should ingore excluded checks when determining health", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"ok1": healthz.Ping,
				"bad1": func(req *http.Request) error {
					return errors.New("blech")
				},
			}}

			resp := requestTo(handler, "/?exclude=bad1")
			Expect(resp.Code).To(Equal(http.StatusOK))
		})

		It("should be fine if asked to exclude a check that doesn't exist", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"ok1": healthz.Ping,
				"ok2": healthz.Ping,
			}}

			resp := requestTo(handler, "/?exclude=nonexistant")
			Expect(resp.Code).To(Equal(http.StatusOK))
		})

		Context("when verbose output is requested with ?verbose=true", func() {
			It("should return verbose output for ok cases", func() {
				handler := &healthz.Handler{Checks: map[string]healthz.Checker{
					"ok1": healthz.Ping,
					"ok2": healthz.Ping,
				}}

				resp := requestTo(handler, "/?verbose=true")
				Expect(resp.Code).To(Equal(http.StatusOK))
				Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
				Expect(resp.Body.String()).To(Equal("[+]ok1 ok\n[+]ok2 ok\nhealthz check passed\n"))
			})

			It("should return verbose output for failures", func() {
				handler := &healthz.Handler{Checks: map[string]healthz.Checker{
					"ok1": healthz.Ping,
					"bad1": func(req *http.Request) error {
						return errors.New("blech")
					},
				}}

				resp := requestTo(handler, "/?verbose=true")
				Expect(resp.Code).To(Equal(http.StatusInternalServerError))
				Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
				Expect(resp.Body.String()).To(Equal("[-]bad1 failed: reason withheld\n[+]ok1 ok\nhealthz check failed\n"))
			})
		})

		It("should return non-verbose output when healthy and not specified as verbose", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"ok1": healthz.Ping,
				"ok2": healthz.Ping,
			}}

			resp := requestTo(handler, "/")
			Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
			Expect(resp.Body.String()).To(Equal("ok"))

		})

		It("should always be verbose if a check fails", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"ok1": healthz.Ping,
				"bad1": func(req *http.Request) error {
					return errors.New("blech")
				},
			}}

			resp := requestTo(handler, "/")
			Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
			Expect(resp.Body.String()).To(Equal("[-]bad1 failed: reason withheld\n[+]ok1 ok\nhealthz check failed\n"))
		})

		It("should always return a ping endpoint if no other ones are present", func() {
			resp := requestTo(&healthz.Handler{}, "/?verbose=true")
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
			Expect(resp.Body.String()).To(Equal("[+]ping ok\nhealthz check passed\n"))
		})
	})

	Describe("the per-check endpoints", func() {
		It("should return ok if the requested check is healthy", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"okcheck": healthz.Ping,
			}}

			resp := requestTo(handler, "/okcheck")
			Expect(resp.Code).To(Equal(http.StatusOK))
			Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
			Expect(resp.Body.String()).To(Equal("ok"))
		})

		It("should return an error if the requested check is unhealthy", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"failcheck": func(req *http.Request) error {
					return errors.New("blech")
				},
			}}

			resp := requestTo(handler, "/failcheck")
			Expect(resp.Code).To(Equal(http.StatusInternalServerError))
			Expect(resp.Header().Get("Content-Type")).To(Equal(contentType))
			Expect(resp.Body.String()).To(Equal("internal server error: blech\n"))
		})

		It("shouldn't take other checks into account", func() {
			handler := &healthz.Handler{Checks: map[string]healthz.Checker{
				"failcheck": func(req *http.Request) error {
					return errors.New("blech")
				},
				"okcheck": healthz.Ping,
			}}

			By("checking the bad endpoint and expecting it to fail")
			resp := requestTo(handler, "/failcheck")
			Expect(resp.Code).To(Equal(http.StatusInternalServerError))

			By("checking the good endpoint and expecting it to succeed")
			resp = requestTo(handler, "/okcheck")
			Expect(resp.Code).To(Equal(http.StatusOK))
		})

		It("should return non-found for paths that don't match a checker", func() {
			handler := &healthz.Handler{}

			resp := requestTo(handler, "/doesnotexist")
			Expect(resp.Code).To(Equal(http.StatusNotFound))
		})

		It("should always return a ping endpoint if no other ones are present", func() {
			resp := requestTo(&healthz.Handler{}, "/ping")
			Expect(resp.Code).To(Equal(http.StatusOK))
		})
	})
})
