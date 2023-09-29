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

package karpenterconvert

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

type testcase struct {
	name       string
	file       string
	outputFile string
}

func TestConvertObject(t *testing.T) {
	testcases := []testcase{
		{
			name:       "provisioner to nodepool",
			file:       "./test/fixtures/provisioner.yaml",
			outputFile: "./test/fixtures/nodepool.yaml",
		},
		{
			name:       "nodetemplate to nodeclass",
			file:       "./test/fixtures/nodetemplate.yaml",
			outputFile: "./test/fixtures/nodeclass.yaml",
		},
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%s", tc.name), func(t *testing.T) {
			tf := cmdtesting.NewTestFactory().WithNamespace("test")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
					return nil, nil
				}),
			}

			buf := bytes.NewBuffer([]byte{})
			cmd := NewCmd(tf, genericiooptions.IOStreams{Out: buf, ErrOut: buf})
			cmd.Flags().Set("filename", tc.file)
			cmd.Flags().Set("local", "true")
			cmd.Flags().Set("output", "yaml")
			cmd.Run(cmd, []string{})

			bytes, _ := os.ReadFile(tc.outputFile)
			content := string(bytes)

			if !strings.Contains(buf.String(), content) {
				t.Errorf("unexpected output when converting %s to %q, expected: %q, but got %q", tc.file, tc.outputFile, content, buf.String())
			}
		})
	}
}
