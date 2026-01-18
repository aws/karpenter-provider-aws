package http

import (
	"fmt"
	"testing"
)

func TestJoinPath(t *testing.T) {
	cases := []struct {
		A, B   string
		Expect string
	}{
		0: {
			A: "", B: "/bar",
			Expect: "/bar",
		},
		1: {
			A: "/", B: "/bar",
			Expect: "/bar",
		},
		2: {
			A: "/foo/", B: "/bar",
			Expect: "/foo/bar",
		},
		3: {
			A: "foo/", B: "/bar",
			Expect: "/foo/bar",
		},
		4: {
			A: "/foo/", B: "bar",
			Expect: "/foo/bar",
		},
		5: {
			A: "foo", B: "/bar",
			Expect: "/foo/bar",
		},
		6: {
			A: "", B: "",
			Expect: "/",
		},
		7: {
			A: "foo", B: "",
			Expect: "/foo",
		},
		8: {
			A: "foo/", B: "",
			Expect: "/foo/",
		},
		9: {
			A: "foo//", B: "//bar",
			Expect: "/foo///bar",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("%d:%s,%s:%s", i, c.A, c.B, c.Expect), func(t *testing.T) {
			actual := JoinPath(c.A, c.B)
			if e, a := c.Expect, actual; e != a {
				t.Errorf("expect %v path, got %v", e, a)
			}
		})
	}
}

func TestJoinRawQuery(t *testing.T) {
	cases := []struct {
		A, B   string
		Expect string
	}{
		0: {
			A: "", B: "bar",
			Expect: "bar",
		},
		1: {
			A: "foo", B: "bar",
			Expect: "foo&bar",
		},
		2: {
			A: "foo&", B: "bar",
			Expect: "foo&bar",
		},
		3: {
			A: "foo", B: "",
			Expect: "foo",
		},
		4: {
			A: "", B: "&bar",
			Expect: "bar",
		},
		5: {
			A: "foo&", B: "&bar",
			Expect: "foo&bar",
		},
		6: {
			A: "", B: "",
			Expect: "",
		},
		7: {
			A: "foo&baz", B: "bar",
			Expect: "foo&baz&bar",
		},
		8: {
			A: "foo", B: "baz&bar",
			Expect: "foo&baz&bar",
		},
		9: {
			A: "&foo&", B: "&baz&bar&",
			Expect: "foo&baz&bar",
		},
		10: {
			A: "&foo&&&", B: "&&&baz&&&bar&",
			Expect: "foo&baz&&&bar",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("%d:%s,%s:%s", i, c.A, c.B, c.Expect), func(t *testing.T) {
			actual := JoinRawQuery(c.A, c.B)
			if e, a := c.Expect, actual; e != a {
				t.Errorf("expect %v query, got %v", e, a)
			}
		})
	}
}
