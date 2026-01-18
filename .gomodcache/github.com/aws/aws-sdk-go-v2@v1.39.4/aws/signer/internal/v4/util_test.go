package v4

import (
	"net/http"
	"net/url"
	"testing"
)

func lazyURLParse(v string) func() (*url.URL, error) {
	return func() (*url.URL, error) {
		return url.Parse(v)
	}
}

func TestGetURIPath(t *testing.T) {
	cases := map[string]struct {
		getURL func() (*url.URL, error)
		expect string
	}{
		// Cases
		"with scheme": {
			getURL: lazyURLParse("https://localhost:9000"),
			expect: "/",
		},
		"no port, with scheme": {
			getURL: lazyURLParse("https://localhost"),
			expect: "/",
		},
		"without scheme": {
			getURL: lazyURLParse("localhost:9000"),
			expect: "/",
		},
		"without scheme, with path": {
			getURL: lazyURLParse("localhost:9000/abc123"),
			expect: "/abc123",
		},
		"without scheme, with separator": {
			getURL: lazyURLParse("//localhost:9000"),
			expect: "/",
		},
		"no port, without scheme, with separator": {
			getURL: lazyURLParse("//localhost"),
			expect: "/",
		},
		"without scheme, with separator, with path": {
			getURL: lazyURLParse("//localhost:9000/abc123"),
			expect: "/abc123",
		},
		"no port, without scheme, with separator, with path": {
			getURL: lazyURLParse("//localhost/abc123"),
			expect: "/abc123",
		},
		"opaque with query string": {
			getURL: lazyURLParse("localhost:9000/abc123?efg=456"),
			expect: "/abc123",
		},
		"failing test": {
			getURL: func() (*url.URL, error) {
				endpoint := "https://service.region.amazonaws.com"
				req, _ := http.NewRequest("POST", endpoint, nil)
				u := req.URL

				u.Opaque = "//example.org/bucket/key-._~,!@#$%^&*()"

				query := u.Query()
				query.Set("some-query-key", "value")
				u.RawQuery = query.Encode()

				return u, nil
			},
			expect: "/bucket/key-._~,!@#$%^&*()",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			u, err := c.getURL()
			if err != nil {
				t.Fatalf("failed to get URL, %v", err)
			}

			actual := GetURIPath(u)
			if e, a := c.expect, actual; e != a {
				t.Errorf("expect %v path, got %v", e, a)
			}
		})
	}
}

func TestStripExcessHeaders(t *testing.T) {
	vals := []string{
		"",
		"123",
		"1 2 3",
		"1 2 3 ",
		"  1 2 3",
		"1  2 3",
		"1  23",
		"1  2  3",
		"1  2  ",
		" 1  2  ",
		"12   3",
		"12   3   1",
		"12           3     1",
		"12     3       1abc123",
	}

	expected := []string{
		"",
		"123",
		"1 2 3",
		"1 2 3",
		"1 2 3",
		"1 2 3",
		"1 23",
		"1 2 3",
		"1 2",
		"1 2",
		"12 3",
		"12 3 1",
		"12 3 1",
		"12 3 1abc123",
	}

	for i := 0; i < len(vals); i++ {
		r := StripExcessSpaces(vals[i])
		if e, a := expected[i], r; e != a {
			t.Errorf("%d, expect %v, got %v", i, e, a)
		}
	}
}

var stripExcessSpaceCases = []string{
	`AWS4-HMAC-SHA256 Credential=AKIDFAKEIDFAKEID/20160628/us-west-2/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=1234567890abcdef1234567890abcdef1234567890abcdef`,
	`123   321   123   321`,
	`   123   321   123   321   `,
	`   123    321    123          321   `,
	"123",
	"1 2 3",
	"  1 2 3",
	"1  2 3",
	"1  23",
	"1  2  3",
	"1  2  ",
	" 1  2  ",
	"12   3",
	"12   3   1",
	"12           3     1",
	"12     3       1abc123",
}

func BenchmarkStripExcessSpaces(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, v := range stripExcessSpaceCases {
			StripExcessSpaces(v)
		}
	}
}
