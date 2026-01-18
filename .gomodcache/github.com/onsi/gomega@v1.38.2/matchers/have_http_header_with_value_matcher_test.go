package matchers_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HaveHTTPHeader", func() {
	It("can match an HTTP header", func() {
		resp := &http.Response{}
		resp.Header = make(http.Header)
		resp.Header.Add("fake-header", "fake value")
		Expect(resp).To(HaveHTTPHeaderWithValue("fake-header", "fake value"))
	})

	It("can mismatch an HTTP header", func() {
		resp := &http.Response{}
		resp.Header = make(http.Header)
		resp.Header.Add("fake-header", "fake value")
		Expect(resp).NotTo(HaveHTTPHeaderWithValue("other-header", "fake value"))
		Expect(resp).NotTo(HaveHTTPHeaderWithValue("fake-header", "other value"))
	})

	When("the header is set more than once", func() {
		It("matches the first value and not the second", func() {
			resp := &http.Response{}
			resp.Header = make(http.Header)
			resp.Header.Add("fake-header", "fake value1")
			resp.Header.Add("fake-header", "fake value2")
			Expect(resp).To(HaveHTTPHeaderWithValue("fake-header", "fake value1"))
			Expect(resp).NotTo(HaveHTTPHeaderWithValue("fake-header", "fake value2"))
		})
	})

	When("ACTUAL is *httptest.ResponseRecorder", func() {
		It("can match an HTTP header", func() {
			resp := &httptest.ResponseRecorder{}
			resp.Header().Add("fake-header", "fake value")
			Expect(resp).To(HaveHTTPHeaderWithValue("fake-header", "fake value"))
		})

		It("can mismatch an HTTP header", func() {
			resp := &httptest.ResponseRecorder{}
			resp.Header().Add("fake-header", "fake value")
			Expect(resp).NotTo(HaveHTTPHeaderWithValue("other-header", "fake value"))
			Expect(resp).NotTo(HaveHTTPHeaderWithValue("fake-header", "other value"))
		})
	})

	When("ACTUAL is neither *http.Response nor *httptest.ResponseRecorder", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				Expect("foo").To(HaveHTTPHeaderWithValue("bar", "baz"))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPHeaderWithValue matcher expects *http.Response or *httptest.ResponseRecorder. Got:\n    <string>: foo"))
		})
	})

	When("EXPECTED VALUE is a matcher", func() {
		It("can match an HTTP header", func() {
			resp := &http.Response{}
			resp.Header = make(http.Header)
			resp.Header.Add("fake-header", "fake value")
			Expect(resp).To(HaveHTTPHeaderWithValue("fake-header", ContainSubstring("value")))
		})

		It("can mismatch an HTTP header", func() {
			resp := &http.Response{}
			resp.Header = make(http.Header)
			resp.Header.Add("fake-header", "fake value")
			Expect(resp).NotTo(HaveHTTPHeaderWithValue("fake-header", ContainSubstring("foo")))
		})
	})

	When("EXPECTED VALUE is something else", func() {
		It("errors", func() {
			failures := InterceptGomegaFailures(func() {
				resp := &http.Response{}
				Expect(resp).To(HaveHTTPHeaderWithValue("bar", 42))
			})
			Expect(failures).To(HaveLen(1))
			Expect(failures[0]).To(Equal("HaveHTTPHeaderWithValue matcher must be passed a string or a GomegaMatcher. Got:\n    <int>: 42"))
		})
	})

	Describe("FailureMessage", func() {
		When("matching a string", func() {
			It("returns message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{}
					resp.Header = make(http.Header)
					resp.Header.Add("fake-header", "fake value")
					Expect(resp).To(HaveHTTPHeaderWithValue("fake-header", "other value"))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`HTTP header "fake-header":
    Expected
        <string>: fake value
    to equal
        <string>: other value`), failures[0])
			})
		})

		When("matching a matcher", func() {
			It("returns message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{}
					resp.Header = make(http.Header)
					resp.Header.Add("fake-header", "fake value")
					Expect(resp).To(HaveHTTPHeaderWithValue("fake-header", ContainSubstring("other")))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`HTTP header "fake-header":
    Expected
        <string>: fake value
    to contain substring
        <string>: other`), failures[0])
			})
		})
	})

	Describe("NegatedFailureMessage", func() {
		When("matching a string", func() {
			It("returns message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{}
					resp.Header = make(http.Header)
					resp.Header.Add("fake-header", "fake value")
					Expect(resp).NotTo(HaveHTTPHeaderWithValue("fake-header", "fake value"))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`HTTP header "fake-header":
    Expected
        <string>: fake value
    not to equal
        <string>: fake value`), failures[0])
			})
		})

		When("matching a matcher", func() {
			It("returns message", func() {
				failures := InterceptGomegaFailures(func() {
					resp := &http.Response{}
					resp.Header = make(http.Header)
					resp.Header.Add("fake-header", "fake value")
					Expect(resp).NotTo(HaveHTTPHeaderWithValue("fake-header", ContainSubstring("value")))
				})
				Expect(failures).To(HaveLen(1))
				Expect(failures[0]).To(Equal(`HTTP header "fake-header":
    Expected
        <string>: fake value
    not to contain substring
        <string>: value`), failures[0])
			})
		})
	})
})
