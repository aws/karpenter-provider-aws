package matchers_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal/gutil"
)

var _ = Describe("HaveHTTPBody", func() {
	When("ACTUAL is *http.Response", func() {
		It("matches the body", func() {
			const body = "this is the body"
			resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
			Expect(resp).To(HaveHTTPBody(body))
		})

		It("mismatches the body", func() {
			const body = "this is the body"
			resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
			Expect(resp).NotTo(HaveHTTPBody("something else"))
		})

		It("matches the body against later calls", func() {
			firstCall := true
			getResp := func() *http.Response {
				if firstCall {
					firstCall = false
					return &http.Response{Body: io.NopCloser(strings.NewReader("first_call"))}
				} else {
					return &http.Response{Body: io.NopCloser(strings.NewReader("later_call"))}
				}
			}
			Eventually(getResp).MustPassRepeatedly(2).Should(HaveHTTPBody([]byte("later_call")))
		})
	})

	When("ACTUAL is *httptest.ResponseRecorder", func() {
		It("matches the body", func() {
			const body = "this is the body"
			resp := &httptest.ResponseRecorder{Body: bytes.NewBufferString(body)}
			Expect(resp).To(HaveHTTPBody(body))
		})

		It("mismatches the body", func() {
			const body = "this is the body"
			resp := &httptest.ResponseRecorder{Body: bytes.NewBufferString(body)}
			Expect(resp).NotTo(HaveHTTPBody("something else"))
		})

		It("matches the body against later calls", func() {
			firstCall := true
			getResp := func() *httptest.ResponseRecorder {
				if firstCall {
					firstCall = false
					return &httptest.ResponseRecorder{Body: bytes.NewBufferString("first_call")}
				} else {
					return &httptest.ResponseRecorder{Body: bytes.NewBufferString("later_call")}
				}
			}
			Eventually(getResp).MustPassRepeatedly(2).Should(HaveHTTPBody([]byte("later_call")))
		})
	})

	When("ACTUAL is neither *http.Response nor *httptest.ResponseRecorder", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				Expect("foo").To(HaveHTTPBody("bar"))
			})
			Expect(failures).To(ConsistOf("HaveHTTPBody matcher expects *http.Response or *httptest.ResponseRecorder. Got:\n    <string>: foo"))
		})
	})

	When("EXPECTED is []byte", func() {
		It("matches the body", func() {
			const body = "this is the body"
			resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
			Expect(resp).To(HaveHTTPBody([]byte(body)))
		})

		It("mismatches the body", func() {
			const body = "this is the body"
			resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
			Expect(resp).NotTo(HaveHTTPBody([]byte("something else")))
		})
	})

	When("EXPECTED is a submatcher", func() {
		It("matches the body", func() {
			resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(`{"some":"json"}`))}
			Expect(resp).To(HaveHTTPBody(MatchJSON(`{ "some": "json" }`)))
		})

		It("mismatches the body", func() {
			resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(`{"some":"json"}`))}
			Expect(resp).NotTo(HaveHTTPBody(MatchJSON(`{ "something": "different" }`)))
		})
	})

	When("EXPECTED is something else", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{Body: gutil.NopCloser(strings.NewReader("body"))}
				Expect(resp).To(HaveHTTPBody(map[int]bool{}))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPBody matcher expects string, []byte, or GomegaMatcher. Got:\n    <map[int]bool | len:0>: {}"))
		})
	})

	Describe("FailureMessage", func() {
		Context("EXPECTED is string", func() {
			It("returns a match failure message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{Body: gutil.NopCloser(strings.NewReader("this is the body"))}
					Expect(resp).To(HaveHTTPBody("this is a different body"))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`Expected
    <string>: this is the body
to equal
    <string>: this is a different body`), failures[0])
			})
		})

		Context("EXPECTED is []byte", func() {
			It("returns a match failure message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{Body: gutil.NopCloser(strings.NewReader("this is the body"))}
					Expect(resp).To(HaveHTTPBody([]byte("this is a different body")))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(MatchRegexp(`^Expected
    <\[\]uint8 \| len:\d+, cap:\d+>: this is the body
to equal
    <\[\]uint8 ]| len:\d+, cap:\d+>: this is a different body$`))
			})
		})

		Context("EXPECTED is submatcher", func() {
			It("returns a match failure message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(`{"some":"json"}`))}
					Expect(resp).To(HaveHTTPBody(MatchJSON(`{"other":"stuff"}`)))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`Expected
    <string>: {
      "some": "json"
    }
to match JSON of
    <string>: {
      "other": "stuff"
    }`))
			})
		})
	})

	Describe("NegatedFailureMessage", func() {
		Context("EXPECTED is string", func() {
			It("returns a negated failure message", func() {
				const body = "this is the body"
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
					Expect(resp).NotTo(HaveHTTPBody(body))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`Expected
    <string>: this is the body
not to equal
    <string>: this is the body`))
			})
		})

		Context("EXPECTED is []byte", func() {
			It("returns a match failure message", func() {
				const body = "this is the body"
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
					Expect(resp).NotTo(HaveHTTPBody([]byte(body)))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(MatchRegexp(`^Expected
    <\[\]uint8 \| len:\d+, cap:\d+>: this is the body
not to equal
    <\[\]uint8 \| len:\d+, cap:\d+>: this is the body$`))
			})
		})

		Context("EXPECTED is submatcher", func() {
			It("returns a match failure message", func() {
				const body = `{"some":"json"}`
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{Body: gutil.NopCloser(strings.NewReader(body))}
					Expect(resp).NotTo(HaveHTTPBody(MatchJSON(body)))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`Expected
    <string>: {
      "some": "json"
    }
not to match JSON of
    <string>: {
      "some": "json"
    }`))
			})
		})
	})
})
