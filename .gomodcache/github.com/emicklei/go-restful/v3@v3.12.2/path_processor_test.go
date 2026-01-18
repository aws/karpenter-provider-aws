package restful

import (
	"testing"
)

func TestMatchesPath_OneParam(t *testing.T) {
	params := doExtractParams("/from/{source}", 2, "/from/here", t)
	if params["source"] != "here" {
		t.Errorf("parameter mismatch here")
	}
}

func TestMatchesPath_Slash(t *testing.T) {
	params := doExtractParams("/", 0, "/", t)
	if len(params) != 0 {
		t.Errorf("expected empty parameters")
	}
}

func TestMatchesPath_SlashNonVar(t *testing.T) {
	params := doExtractParams("/any", 1, "/any", t)
	if len(params) != 0 {
		t.Errorf("expected empty parameters")
	}
}

func TestMatchesPath_TwoVars(t *testing.T) {
	params := doExtractParams("/from/{source}/to/{destination}", 4, "/from/AMS/to/NY", t)
	if params["source"] != "AMS" {
		t.Errorf("parameter mismatch AMS")
	}
}

func TestMatchesPath_VarOnFront(t *testing.T) {
	params := doExtractParams("{what}/from/{source}/", 3, "who/from/SOS/", t)
	if params["source"] != "SOS" {
		t.Errorf("parameter mismatch SOS")
	}
}

func TestMatchesPath_VarOnFront_KeepSlash(t *testing.T) {
	TrimRightSlashEnabled = false
	params := doExtractParams("{what}/from/{source}/", 4, "who/from/SOS/", t)
	if params["source"] != "SOS" {
		t.Errorf("parameter mismatch SOS")
	}
}

func TestExtractParameters_EmptyValue(t *testing.T) {
	params := doExtractParams("/fixed/{var}", 2, "/fixed/", t)
	if params["var"] != "" {
		t.Errorf("parameter mismatch var")
	}
}

func TestExtractParameters_Dot(t *testing.T) {
	params := doExtractParams("/fixed/{var}.foo", 2, "/fixed/barrr.foo", t)
	if params["var"] != "barrr" {
		t.Errorf("parameter mismatch var")
	}
}

func TestExtractParameters_Prefix(t *testing.T) {
	params := doExtractParams("/fixed/foo_{var}", 2, "/fixed/foo_barrr", t)
	if params["var"] != "barrr" {
		t.Errorf("parameter mismatch var")
	}
}

func TestExtractParameters_Suffix(t *testing.T) {
	params := doExtractParams("/fixed/{var}_foo", 2, "/fixed/barrr_foo", t)
	if params["var"] != "barrr" {
		t.Errorf("parameter mismatch var")
	}
}

func TestExtractParameters_Mixed(t *testing.T) {
	params := doExtractParams("/fixed/foo_{var}_bar", 2, "/fixed/foo_barrr_bar", t)
	if params["var"] != "barrr" {
		t.Errorf("parameter mismatch var")
	}
}

func TestExtractParameters_RegexAndCustomVerb(t *testing.T) {
	testCase := []struct {
		route     string
		size      int
		path      string
		checkList map[string]string
	}{
		{"/projects/{projectId}/users/{id:^prefix-}:custom", 4, "/projects/110/users/prefix-userId:custom", map[string]string{
			"projectId": "110",
			"id":        "prefix-userId"}},
		{"/projects/{projectId}/users/{id:*}", 4, "/projects/110/users/prefix-userId:custom", map[string]string{
			"projectId": "110",
			"id":        "prefix-userId:custom"}},
	}

	for idx, v := range testCase {
		params := doExtractParams(v.route, v.size, v.path, t)
		for k, val := range v.checkList {
			if params[k] != val {
				t.Errorf("[%v] params: %v mismatch, expected: %v, actual: %v", idx, k, v.checkList[k], params[k])
			}
		}
	}
}

func doExtractParams(routePath string, size int, urlPath string, t *testing.T) map[string]string {
	r := Route{Path: routePath}
	r.postBuild()
	if len(r.pathParts) != size {
		for i, each := range r.pathParts {
			t.Logf("%d:%q", i, each)
		}
		t.Fatalf("len not %v, but %v", size, len(r.pathParts))
	}
	pathProcessor := defaultPathProcessor{}
	return pathProcessor.ExtractParameters(&r, nil, urlPath)
}
