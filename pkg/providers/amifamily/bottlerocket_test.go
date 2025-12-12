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

package amifamily

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("Bottlerocket", func() {
	Describe("resolveAMIVersion", func() {
		It("should resolve version from alias", func() {
			b := Bottlerocket{
				Options: &Options{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{Alias: "bottlerocket@v1.46.0"},
					},
				},
			}
			result := b.resolveAMIVersion()
			Expect(result).To(Equal("v1.46.0"))
		})

		It("should resolve latest from alias", func() {
			b := Bottlerocket{
				Options: &Options{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{Alias: "bottlerocket@latest"},
					},
				},
			}
			result := b.resolveAMIVersion()
			Expect(result).To(Equal("latest"))
		})

		It("should resolve version from name", func() {
			b := Bottlerocket{
				Options: &Options{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{Name: "bottlerocket-aws-k8s-1.33-x86_64-v1.46.0-431fe75a"},
					},
				},
			}
			result := b.resolveAMIVersion()
			Expect(result).To(Equal("v1.46.0"))
		})

		It("should resolve version from ID with resolved AMI", func() {
			b := Bottlerocket{
				Options: &Options{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{ID: "ami-12345"},
					},
					AMIs: []v1.AMI{
						{ID: "ami-12345", Name: "bottlerocket-aws-k8s-1.33-x86_64-v1.47.0-abc123"},
					},
				},
			}
			result := b.resolveAMIVersion()
			Expect(result).To(Equal("v1.47.0"))
		})

		It("should resolve version from resolved AMI fallback", func() {
			b := Bottlerocket{
				Options: &Options{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{Tags: map[string]string{"Environment": "prod"}},
					},
					AMIs: []v1.AMI{
						{ID: "ami-67890", Name: "bottlerocket-aws-k8s-1.30-arm64-v2.0.1-def456"},
					},
				},
			}
			result := b.resolveAMIVersion()
			Expect(result).To(Equal("v2.0.1"))
		})

		It("should return empty when no version found", func() {
			b := Bottlerocket{
				Options: &Options{
					AMISelectorTerms: []v1.AMISelectorTerm{
						{Name: "some-other-ami"},
					},
				},
			}
			result := b.resolveAMIVersion()
			Expect(result).To(Equal(""))
		})
	})
})
