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

package v1_test

import (
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Alias", func() {
	It("should resolve the family and version from a well-formed alias", func() {
		nodeClass := &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{
			AMISelectorTerms: []v1.AMISelectorTerm{{Alias: "bottlerocket@v1.46.0"}},
		}}
		Expect(nodeClass.Alias()).To(Equal(&v1.Alias{Family: v1.AMIFamilyBottlerocket, Version: "v1.46.0"}))
		Expect(nodeClass.AMIFamily()).To(Equal(v1.AMIFamilyBottlerocket))
	})
	It("should return nil when no term specifies an alias", func() {
		nodeClass := &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{
			AMIFamily:        &v1.AMIFamilyAL2023,
			AMISelectorTerms: []v1.AMISelectorTerm{{ID: "ami-1234749"}},
		}}
		Expect(nodeClass.Alias()).To(BeNil())
		Expect(nodeClass.AMIFamily()).To(Equal(v1.AMIFamilyAL2023))
	})
	DescribeTable("should not resolve an alias it cannot parse",
		func(alias string) {
			nodeClass := &v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{
				AMISelectorTerms: []v1.AMISelectorTerm{{Alias: alias}},
			}}
			Expect(nodeClass.Alias()).To(BeNil())
			Expect(nodeClass.AMIFamily()).To(Equal(v1.AMIFamilyCustom))
		},
		Entry("extra separator", "al2023@latest@foo"),
		Entry("missing separator", "al2023"),
		Entry("unknown family", "ubuntu@latest"),
	)
})
