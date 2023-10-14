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

package convert_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter/tools/karpenter-convert/pkg/convert"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func TestConvert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Karpenter-Convert")
}

var _ = Describe("ConvertObject", func() {
	type testcase struct {
		name           string
		ignoreDefaults bool
		file           string
		outputFile     string
	}

	DescribeTable("conversion tests",
		func(tc testcase) {
			tf := cmdtesting.NewTestFactory().WithNamespace("test")
			defer tf.Cleanup()

			tf.UnstructuredClient = &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					Fail(fmt.Sprintf("unexpected request: %#v\n%#v", req.URL, req))
					return nil, nil
				}),
			}

			buf := bytes.NewBuffer([]byte{})
			cmd := convert.NewCmd(tf, genericiooptions.IOStreams{Out: buf, ErrOut: buf})
			if err := cmd.Flags().Set("filename", tc.file); err != nil {
				Expect(err).To(BeNil())
			}
			if err := cmd.Flags().Set("output", "yaml"); err != nil {
				Expect(err).To(BeNil())
			}
			if err := cmd.Flags().Set("ignore-defaults", strconv.FormatBool(tc.ignoreDefaults)); err != nil {
				Expect(err).To(BeNil())
			}

			cmd.Run(cmd, []string{})

			bytes, _ := os.ReadFile(tc.outputFile)
			content := string(bytes)

			Expect(buf.String()).To(Equal(content), fmt.Sprintf("unexpected output when converting %s to %q, expected: %q, but got %q", tc.file, tc.outputFile, content, buf.String()))
		},

		Entry("provisioner to nodepool",
			testcase{
				name:       "provisioner to nodepool",
				file:       "./testdata/provisioner.yaml",
				outputFile: "./testdata/nodepool.yaml",
			},
		),

		Entry("provisioner (set defaults) to nodepool",
			testcase{
				name:           "provisioner to nodepool",
				file:           "./testdata/provisioner_defaults.yaml",
				outputFile:     "./testdata/nodepool_defaults.yaml",
				ignoreDefaults: false,
			},
		),

		Entry("provisioner (no set defaults) to nodepool",
			testcase{
				name:           "provisioner to nodepool",
				file:           "./testdata/provisioner_no_defaults.yaml",
				outputFile:     "./testdata/nodepool_no_defaults.yaml",
				ignoreDefaults: true,
			},
		),

		Entry("nodetemplate to nodeclass",
			testcase{
				name:       "nodetemplate to nodeclass",
				file:       "./testdata/nodetemplate.yaml",
				outputFile: "./testdata/nodeclass.yaml",
			},
		),
		Entry("nodetemplate (empty amifamily) to nodeclass",
			testcase{
				name:       "nodetemplate (empty amifamily) to nodeclass",
				file:       "./testdata/nodetemplate_no_amifamily.yaml",
				outputFile: "./testdata/nodeclass_no_amifamily.yaml",
			},
		),
	)
})
