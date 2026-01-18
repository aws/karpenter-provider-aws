package restful

import "testing"

func TestHasCustomVerb(t *testing.T) {
	testCase := []struct {
		path string
		has  bool
	}{
		{"/{userId}:init", true},
		{"/{userId:init}", false},
		{"/users/{id:init}:init", true},
		{"/users/{id}", false},
	}

	for _, v := range testCase {
		rs := hasCustomVerb(v.path)
		if rs != v.has {
			t.Errorf("path: %v should has no custom verb", v.path)
		}
	}
}

func TestRemoveCustomVerb(t *testing.T) {
	testCase := []struct {
		path         string
		expectedPath string
	}{
		{"/{userId}:init", "/{userId}"},
		{"/{userId:init}", "/{userId:init}"},
		{"/users/{id:init}:init", "/users/{id:init}"},
		{"/users/{id}", "/users/{id}"},
		{"/init/users/{id:init}:init", "/init/users/{id:init}"},
	}

	for _, v := range testCase {
		rs := removeCustomVerb(v.path)
		if rs != v.expectedPath {
			t.Errorf("expected value: %v, actual: %v", v.expectedPath, rs)
		}
	}
}
func TestMatchCustomVerb(t *testing.T) {
	testCase := []struct {
		routeToken string
		pathToken  string
		expected   bool
	}{
		{"{userId:regex}:init", "creator-123456789gWe:init", true},
		{"{userId:regex}:init", "creator-123456789gWe", false},
		{"{userId:regex}", "creator-123456789gWe:init", false},
		{"users:init", "users:init", true},
		{"users:init", "tokens:init", true},
	}

	for idx, v := range testCase {
		rs := isMatchCustomVerb(v.routeToken, v.pathToken)
		if rs != v.expected {
			t.Errorf("expected value: %v, actual: %v, index: [%v]", v.expected, rs, idx)
		}
	}
}
