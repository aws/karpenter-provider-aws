/*
Copyright 2021 The Kubernetes Authors.

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

package spec3_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"k8s.io/kube-openapi/pkg/spec3"
	jsontesting "k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestSecuritySchemeRoundTrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			Name: "Basic Roundtrip",
			Object: &spec3.SecurityScheme{
				spec.Refable{Ref: spec.MustCreateRef("Dog")},
				spec3.SecuritySchemeProps{
					Description: "foo",
				},
				spec.VendorExtensible{Extensions: spec.Extensions{
					"x-framework": "go-swagger",
				}},
			},
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&spec3.SecurityScheme{}))
		})
	}
}

func TestSecuritySchemaJSONSerialization(t *testing.T) {
	cases := []struct {
		name           string
		target         *spec3.SecurityScheme
		expectedOutput string
	}{
		// scenario 1
		{
			name: "scenario1: basic authentication",
			target: &spec3.SecurityScheme{
				SecuritySchemeProps: spec3.SecuritySchemeProps{
					Type:   "http",
					Scheme: "basic",
				},
			},
			expectedOutput: `{"type":"http","scheme":"basic"}`,
		},

		// scenario 2
		{
			name: "scenario2: JWT Bearer",
			target: &spec3.SecurityScheme{
				SecuritySchemeProps: spec3.SecuritySchemeProps{
					Type:         "http",
					Scheme:       "basic",
					BearerFormat: "JWT",
				},
			},
			expectedOutput: `{"type":"http","scheme":"basic","bearerFormat":"JWT"}`,
		},

		// scenario 3
		{
			name: "scenario3: implicit OAuth2",
			target: &spec3.SecurityScheme{
				SecuritySchemeProps: spec3.SecuritySchemeProps{
					Type: "oauth2",
					Flows: map[string]*spec3.OAuthFlow{
						"implicit": {
							OAuthFlowProps: spec3.OAuthFlowProps{
								AuthorizationUrl: "https://example.com/api/oauth/dialog",
								Scopes: map[string]string{
									"write:pets": "modify pets in your account",
									"read:pets":  "read your pets",
								},
							},
						},
					},
				},
			},
			expectedOutput: `{"type":"oauth2","flows":{"implicit":{"authorizationUrl":"https://example.com/api/oauth/dialog","scopes":{"read:pets":"read your pets","write:pets":"modify pets in your account"}}}}`,
		},

		// scenario 4
		{
			name: "scenario4: reference Object",
			target: &spec3.SecurityScheme{
				Refable: spec.Refable{Ref: spec.MustCreateRef("k8s.io/api/foo/v1beta1b.bar")},
			},
			expectedOutput: `{"$ref":"k8s.io/api/foo/v1beta1b.bar"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawTarget, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			serializedTarget := string(rawTarget)
			if !cmp.Equal(serializedTarget, tc.expectedOutput) {
				t.Fatalf("diff %s", cmp.Diff(serializedTarget, tc.expectedOutput))
			}
		})
	}
}
