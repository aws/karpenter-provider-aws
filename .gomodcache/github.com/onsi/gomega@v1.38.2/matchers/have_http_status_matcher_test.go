package matchers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal/gutil"
)

var _ = Describe("HaveHTTPStatus", func() {
	When("EXPECTED is single integer", func() {
		It("matches the StatusCode", func() {
			resp := &http.Response{StatusCode: http.StatusOK}
			Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			Expect(resp).NotTo(HaveHTTPStatus(http.StatusNotFound))
		})
	})

	When("EXPECTED is single string", func() {
		It("matches the Status", func() {
			resp := &http.Response{Status: "200 OK"}
			Expect(resp).To(HaveHTTPStatus("200 OK"))
			Expect(resp).NotTo(HaveHTTPStatus("404 Not Found"))
		})
	})

	When("EXPECTED is empty", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{StatusCode: http.StatusOK}
				Expect(resp).To(HaveHTTPStatus())
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPStatus matcher must be passed an int or a string. Got nothing"))
		})
	})

	When("EXPECTED is not a string or integer", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{StatusCode: http.StatusOK}
				Expect(resp).To(HaveHTTPStatus(true))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPStatus matcher must be passed int or string types. Got:\n    <bool>: true"))
		})
	})

	When("EXPECTED is a list of strings and integers", func() {
		It("matches the StatusCode and Status", func() {
			resp := &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
			}
			Expect(resp).To(HaveHTTPStatus(http.StatusOK, http.StatusNoContent, http.StatusNotFound))
			Expect(resp).To(HaveHTTPStatus("204 Feeling Fine", "200 OK", "404 Not Found"))
			Expect(resp).To(HaveHTTPStatus("204 Feeling Fine", http.StatusOK, "404 Not Found"))
			Expect(resp).To(HaveHTTPStatus(http.StatusNoContent, "200 OK", http.StatusNotFound))
			Expect(resp).NotTo(HaveHTTPStatus(http.StatusNotFound, http.StatusNoContent, http.StatusGone))
			Expect(resp).NotTo(HaveHTTPStatus("204 Feeling Fine", "201 Sleeping", "404 Not Found"))
			Expect(resp).NotTo(HaveHTTPStatus(http.StatusNotFound, "404 Not Found", http.StatusGone))
		})
	})

	When("EXPECTED is a list containing non-string or integer types", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{StatusCode: http.StatusOK}
				Expect(resp).To(HaveHTTPStatus(http.StatusGone, "204 No Content", true, http.StatusNotFound))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPStatus matcher must be passed int or string types. Got:\n    <bool>: true"))
		})
	})

	When("ACTUAL is *httptest.ResponseRecorder", func() {
		When("EXPECTED is integer", func() {
			It("matches the StatusCode", func() {
				resp := &httptest.ResponseRecorder{Code: http.StatusOK}
				Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				Expect(resp).NotTo(HaveHTTPStatus(http.StatusNotFound))
			})
		})

		When("EXPECTED is string", func() {
			It("matches the Status", func() {
				resp := &httptest.ResponseRecorder{Code: http.StatusOK}
				Expect(resp).To(HaveHTTPStatus("200 OK"))
				Expect(resp).NotTo(HaveHTTPStatus("404 Not Found"))
			})
		})

		When("EXPECTED is anything else", func() {
			It("does not match", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &httptest.ResponseRecorder{Code: http.StatusOK}
					Expect(resp).NotTo(HaveHTTPStatus(nil))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal("HaveHTTPStatus matcher must be passed int or string types. Got:\n    <nil>: nil"))
			})
		})
	})

	When("ACTUAL is neither *http.Response nor *httptest.ResponseRecorder", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				Expect("foo").To(HaveHTTPStatus(http.StatusOK))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPStatus matcher expects *http.Response or *httptest.ResponseRecorder. Got:\n    <string>: foo"))
		})
	})

	Describe("FailureMessage", func() {
		It("returns a message for a single expected value", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{
					StatusCode: http.StatusBadGateway,
					Status:     "502 Bad Gateway",
					Body:       gutil.NopCloser(strings.NewReader("did not like it")),
				}
				Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal(`Expected
    <*http.Response>: {
        Status:     <string>: "502 Bad Gateway"
        StatusCode: <int>: 502
        Body:       <string>: "did not like it"
    }
to have HTTP status
    <int>: 200`), failures[0])
		})

		It("returns a message for a multiple expected values", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{
					StatusCode: http.StatusBadGateway,
					Status:     "502 Bad Gateway",
					Body:       gutil.NopCloser(strings.NewReader("did not like it")),
				}
				Expect(resp).To(HaveHTTPStatus(http.StatusOK, http.StatusNotFound, "204 No content"))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal(`Expected
    <*http.Response>: {
        Status:     <string>: "502 Bad Gateway"
        StatusCode: <int>: 502
        Body:       <string>: "did not like it"
    }
to have HTTP status
    <int>: 200
    <int>: 404
    <string>: 204 No content`), failures[0])
		})
	})

	Describe("NegatedFailureMessage", func() {
		It("returns a message for a single expected value", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       gutil.NopCloser(strings.NewReader("got it!")),
				}
				Expect(resp).NotTo(HaveHTTPStatus(http.StatusOK))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal(`Expected
    <*http.Response>: {
        Status:     <string>: "200 OK"
        StatusCode: <int>: 200
        Body:       <string>: "got it!"
    }
not to have HTTP status
    <int>: 200`), failures[0])
		})

		It("returns a message for a multiple expected values", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Body:       gutil.NopCloser(strings.NewReader("got it!")),
				}
				Expect(resp).NotTo(HaveHTTPStatus(http.StatusOK, "204 No content", http.StatusGone))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal(`Expected
    <*http.Response>: {
        Status:     <string>: "200 OK"
        StatusCode: <int>: 200
        Body:       <string>: "got it!"
    }
not to have HTTP status
    <int>: 200
    <string>: 204 No content
    <int>: 410`), failures[0])
		})
	})
})
