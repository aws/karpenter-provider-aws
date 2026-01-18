package parser

import "testing"

func Test_getModulePath(t *testing.T) {
	tests := map[string]struct {
		goModPath string
		want      string
	}{
		"valid go.mod without comments and deps": {
			goModPath: "./testdata/default.go.mod",
			want:      "example.com/user/project",
		},
		"valid go.mod with comments and without deps": {
			goModPath: "./testdata/comments.go.mod",
			want:      "example.com/user/project",
		},
		"valid go.mod with comments and deps": {
			goModPath: "./testdata/comments_deps.go.mod",
			want:      "example.com/user/project",
		},
		"actual easyjson go.mod": {
			goModPath: "../go.mod",
			want:      "github.com/mailru/easyjson",
		},
		"invalid go.mod with missing module": {
			goModPath: "./testdata/missing_module.go",
			want:      "",
		},
	}
	for name := range tests {
		tt := tests[name]
		t.Run(name, func(t *testing.T) {
			if got := getModulePath(tt.goModPath); got != tt.want {
				t.Errorf("getModulePath() = %v, want %v", got, tt.want)
			}
		})
	}
}
