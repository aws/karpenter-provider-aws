package http

import "testing"

func TestUserAgentBuilder(t *testing.T) {
	b := NewUserAgentBuilder()
	b.AddKeyValue("foo", "1.2.3")
	b.AddKey("baz")
	if e, a := "foo/1.2.3 baz", b.Build(); e != a {
		t.Errorf("expect %v, got %v", e, a)
	}
}
