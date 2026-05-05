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
		Expect(cpuOptions(nil)).To(BeNil())
	})
	It("should return nil for empty CPUOptions", func() {
		Expect(cpuOptions(&v1.CPUOptions{})).To(BeNil())
	})
	It("should set NestedVirtualization when enabled", func() {
		result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled")})
		Expect(result).ToNot(BeNil())
		Expect(result.NestedVirtualization).To(Equal(ec2types.NestedVirtualizationSpecification("enabled")))
	})
	It("should set NestedVirtualization when disabled", func() {
		result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("disabled")})
		Expect(result).ToNot(BeNil())
		Expect(result.NestedVirtualization).To(Equal(ec2types.NestedVirtualizationSpecification("disabled")))
	})
})
