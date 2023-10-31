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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/aws/karpenter/tools/karpenter-convert/pkg/convert"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func TestConvert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Karpenter-Convert")
}

var _ = Describe("Convert", func() {
	var buf *bytes.Buffer
	var context convert.Context
	BeforeEach(func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()
		tf.UnstructuredClient = &fake.RESTClient{
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				Fail(fmt.Sprintf("unexpected request: %#v\n%#v", req.URL, req))
				return nil, nil
			}),
		}
		buf = bytes.NewBuffer([]byte{})
		context = convert.Context{
			IOStreams: genericiooptions.IOStreams{Out: buf, ErrOut: buf},
			Builder:   tf.NewBuilder,
			Printer:   &printers.YAMLPrinter{},
		}
	})
	DescribeTable("Conversion",
		func(inputFile, outputFile string, ignoreDefaults bool) {
			context.FilenameOptions.Filenames = []string{inputFile}
			context.IgnoreDefaults = ignoreDefaults
			Expect(context.RunConvert()).To(Succeed())
			bytes, _ := os.ReadFile(outputFile)
			content := string(bytes)
			Expect(buf.String()).To(Equal(content))
		},
		Entry("provisioner to nodepool",
			"./testdata/provisioner.yaml",
			"./testdata/nodepool.yaml",
			false,
		),
		Entry("provisioner (set defaults) to nodepool",
			"./testdata/provisioner_defaults.yaml",
			"./testdata/nodepool_defaults.yaml",
			false,
		),
		Entry("provisioner (no set defaults) to nodepool",
			"./testdata/provisioner_no_defaults.yaml",
			"./testdata/nodepool_no_defaults.yaml",
			true,
		),
		Entry("provisioner (kubectl output) to nodepool",
			"./testdata/provisioner_kubectl_output.yaml",
			"./testdata/nodepool_kubectl_output.yaml",
			true,
		),
		Entry("nodetemplate to nodeclass",
			"./testdata/nodetemplate.yaml",
			"./testdata/nodeclass.yaml",
			false,
		),
		Entry("nodetemplate (empty amifamily) to nodeclass",
			"./testdata/nodetemplate_no_amifamily.yaml",
			"./testdata/nodeclass_no_amifamily.yaml",
			false,
		),
		Entry("nodetemplate (kubectl output) to nodeclass",
			"./testdata/nodetemplate_kubectl_output.yaml",
			"./testdata/nodeclass_kubectl_output.yaml",
			false,
		),
	)
	It("should error when converting an AWSNodeTemplate with launchTemplateName", func() {
		context.FilenameOptions.Filenames = []string{"./testdata/nodetemplate_launch_template_name.yaml"}
		err := context.RunConvert()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(`converting AWSNodeTemplate, cannot convert with "spec.launchTemplate"`))
	})
	It("should ignore errors when converting an unknown kind alongside a valid kind", func() {
		context.FilenameOptions.Filenames = []string{"./testdata/unknown_kind.yaml", "./testdata/provisioner.yaml"}
		err := context.RunConvert()
		Expect(err).To(Succeed())

		bytes, _ := os.ReadFile("./testdata/nodepool.yaml")
		content := string(bytes)
		Expect(buf.String()).To(Equal(content))
	})
})

var _ = Describe("CLI Flags", func() {
	var tf *cmdtesting.TestFactory
	var buf *bytes.Buffer
	BeforeEach(func() {
		tf = cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()
		tf.UnstructuredClient = &fake.RESTClient{
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				Fail(fmt.Sprintf("unexpected request: %#v\n%#v", req.URL, req))
				return nil, nil
			}),
		}
		buf = bytes.NewBuffer([]byte{})
	})
	It("should succeed to pass the file option", func() {
		cmd := convert.NewCmd(tf, genericiooptions.IOStreams{Out: buf, ErrOut: buf})
		Expect(cmd.Flags().Set("filename", "./testdata/provisioner.yaml")).To(Succeed())
		cmd.Run(nil, nil)
		bytes, _ := os.ReadFile("./testdata/nodepool.yaml")
		content := string(bytes)
		Expect(buf.String()).To(Equal(content))
	})
	It("should succeed to pass the ignore-defaults option", func() {
		cmd := convert.NewCmd(tf, genericiooptions.IOStreams{Out: buf, ErrOut: buf})
		Expect(cmd.Flags().Set("filename", "./testdata/provisioner_no_defaults.yaml")).To(Succeed())
		Expect(cmd.Flags().Set("ignore-defaults", "true")).To(Succeed())
		cmd.Run(nil, nil)
		bytes, _ := os.ReadFile("./testdata/nodepool_no_defaults.yaml")
		content := string(bytes)
		Expect(buf.String()).To(Equal(content))
		fmt.Println(buf.String())
	})
})
