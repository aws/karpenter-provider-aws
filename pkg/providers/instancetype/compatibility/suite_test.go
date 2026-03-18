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

package compatibility_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
)

func TestCompatibility(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compatibility")
}

var _ = Describe("CompatibilityTest", func() {
	Context("AMIFamilyCompatibility", func() {
		DescribeTable("should handle various instance types across different AMI families",
			func(instanceType string, amiFamily string, expected bool) {
				info := makeInstanceTypeInfo(instanceType)
				nc := newMockNodeClass(amiFamily)
				result := compatibility.IsCompatibleWithNodeClass(info, nc)
				Expect(result).To(Equal(expected))
			},
			Entry("a1.medium w/ Custom AMI", "a1.medium", v1.AMIFamilyCustom, true),
			Entry("a1.large w/ Custom AMI", "a1.large", v1.AMIFamilyCustom, true),
			Entry("t3.medium w/ Custom AMI", "t3.medium", v1.AMIFamilyCustom, true),
			Entry("a1.medium w/ AL2023 AMI", "a1.medium", v1.AMIFamilyAL2023, false),
			Entry("t3.medium w/ AL2023 AMI", "t3.medium", v1.AMIFamilyAL2023, true),
			Entry("a1.large w/ Bottlerocket", "a1.large", v1.AMIFamilyBottlerocket, true),
		)
	})
})

func newMockNodeClass(amiFamily string) *mockNodeClass {
	return &mockNodeClass{
		amiFamily: amiFamily,
	}
}

type mockNodeClass struct {
	amiFamily string
}

func (m mockNodeClass) AMIFamily() string {
	return m.amiFamily
}

func makeInstanceTypeInfo(instanceType string) ec2types.InstanceTypeInfo {
	return ec2types.InstanceTypeInfo{
		InstanceType: ec2types.InstanceType(instanceType),
		ProcessorInfo: &ec2types.ProcessorInfo{
			SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
		},
		VCpuInfo: &ec2types.VCpuInfo{
			DefaultVCpus: aws.Int32(2),
		},
		MemoryInfo: &ec2types.MemoryInfo{
			SizeInMiB: aws.Int64(4096),
		},
	}
}
