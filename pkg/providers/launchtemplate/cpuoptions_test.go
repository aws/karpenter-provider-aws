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

package launchtemplate

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("cpuOptions", func() {
	It("should return nil for nil input", func() {
		Expect(cpuOptions(nil, 0)).To(BeNil())
	})
	It("should return nil for empty CPUOptions", func() {
		Expect(cpuOptions(&v1.CPUOptions{}, 0)).To(BeNil())
	})
	It("should set NestedVirtualization when enabled", func() {
		result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled")}, 0)
		Expect(result).ToNot(BeNil())
		Expect(result.NestedVirtualization).To(Equal(ec2types.NestedVirtualizationSpecification("enabled")))
		Expect(result.CoreCount).To(BeNil())
		Expect(result.ThreadsPerCore).To(BeNil())
	})
	It("should set NestedVirtualization when disabled", func() {
		result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("disabled")}, 0)
		Expect(result).ToNot(BeNil())
		Expect(result.NestedVirtualization).To(Equal(ec2types.NestedVirtualizationSpecification("disabled")))
	})
	It("should set CoreCount and ThreadsPerCore together when a core count is resolved", func() {
		result := cpuOptions(&v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, 8)
		Expect(result).ToNot(BeNil())
		Expect(lo.FromPtr(result.CoreCount)).To(Equal(int32(8)))
		Expect(lo.FromPtr(result.ThreadsPerCore)).To(Equal(int32(1)))
		Expect(result.NestedVirtualization).To(BeEmpty())
	})
	It("should return nil when threadsPerCore is set but the instance types launch with their default layout", func() {
		Expect(cpuOptions(&v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, 0)).To(BeNil())
	})
	It("should not set CoreCount when threadsPerCore is unset", func() {
		result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled")}, 8)
		Expect(result).ToNot(BeNil())
		Expect(result.CoreCount).To(BeNil())
		Expect(result.ThreadsPerCore).To(BeNil())
	})
	It("should combine NestedVirtualization with CoreCount and ThreadsPerCore", func() {
		result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled"), ThreadsPerCore: lo.ToPtr(int32(1))}, 4)
		Expect(result).ToNot(BeNil())
		Expect(result.NestedVirtualization).To(Equal(ec2types.NestedVirtualizationSpecificationEnabled))
		Expect(lo.FromPtr(result.CoreCount)).To(Equal(int32(4)))
		Expect(lo.FromPtr(result.ThreadsPerCore)).To(Equal(int32(1)))
	})
})
