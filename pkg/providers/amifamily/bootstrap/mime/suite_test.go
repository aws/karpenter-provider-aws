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

package mime_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap/mime"
)

var ctx context.Context

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "MIME Parser")
}

var _ = Describe("MIME Parser", func() {
	It("should fail to parse MIME archive with a malformed header", func() {
		content, err := os.ReadFile("test_data/mime_bad_header.txt")
		Expect(err).To(BeNil())
		_, err = mime.NewArchive(string(content))
		Expect(err).ToNot(BeNil())
	})
	It("should successfully parse a valid MIME archive", func() {
		content, err := os.ReadFile("test_data/mime_valid.txt")
		Expect(err).To(BeNil())
		archive, err := mime.NewArchive(string(content))
		Expect(err).To(BeNil())
		Expect(len(archive)).To(Equal(2))
		for ct, f := range map[mime.ContentType]string{
			mime.ContentTypeNodeConfig:  "test_data/nodeconfig.txt",
			mime.ContentTypeShellScript: "test_data/shell.txt",
		} {
			entry, ok := lo.Find(archive, func(e mime.Entry) bool {
				return e.ContentType == ct
			})
			Expect(ok).To(BeTrue())
			expectedContent, err := os.ReadFile(f)
			Expect(err).To(BeNil())
			Expect(entry.Content).To(Equal(string(expectedContent)))
		}
	})
	It("should successfully serialize a MIME archive", func() {
		type entryTemplate struct {
			contentType mime.ContentType
			file        string
		}
		archive := mime.Archive(lo.Map([]entryTemplate{
			{
				contentType: mime.ContentTypeNodeConfig,
				file:        "test_data/nodeconfig.txt",
			},
			{
				contentType: mime.ContentTypeShellScript,
				file:        "test_data/shell.txt",
			},
		}, func(e entryTemplate, _ int) mime.Entry {
			content, err := os.ReadFile(e.file)
			Expect(err).To(BeNil())
			return mime.Entry{
				ContentType: e.contentType,
				Content:     string(content),
			}
		}))
		fmt.Printf("%v\n", archive)
		expected, err := os.ReadFile("test_data/mime_valid.txt")
		Expect(err).To(BeNil())
		encoded, err := archive.Serialize()
		Expect(err).To(BeNil())
		serialized, err := base64.StdEncoding.DecodeString(encoded)
		Expect(err).To(BeNil())
		Expect(string(serialized)).To(Equal(string(expected)))
	})
})
