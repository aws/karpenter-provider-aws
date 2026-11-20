/*
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

package main

import (
	"reflect"
	"testing"
)

func TestGetRule(t *testing.T) {
	versions := []string{"docs", "preview", "v1.0"}
	for _, tc := range []struct {
		name           string
		source, target string
		want           []RedirectRule
	}{
		{
			name:   "a source without a wildcard is kept as is",
			source: "/docs/tasks/amitasks", target: "/docs/tasks/managing-amis",
			want: []RedirectRule{{Source: "/docs/tasks/amitasks", Target: "/docs/tasks/managing-amis", Status: "301"}},
		},
		{
			name:   "* is expanded to every version directory except the target's",
			source: "/*/upgrading/v1beta1-migration", target: "/v1.0/upgrading/v1beta1-migration",
			want: []RedirectRule{
				{Source: "/docs/upgrading/v1beta1-migration", Target: "/v1.0/upgrading/v1beta1-migration", Status: "301"},
				{Source: "/preview/upgrading/v1beta1-migration", Target: "/v1.0/upgrading/v1beta1-migration", Status: "301"},
			},
		},
		{
			name:   "the Amplify <*> wildcard is passed through",
			source: "/v1.12/<*>", target: "/docs/<*>",
			want: []RedirectRule{{Source: "/v1.12/<*>", Target: "/docs/<*>", Status: "301"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := getRule(tc.source, tc.target, versions); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("getRule(%q, %q) = %+v, want %+v", tc.source, tc.target, got, tc.want)
			}
		})
	}
}
