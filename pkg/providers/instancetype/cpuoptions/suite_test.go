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

package cpuoptions_test

import (
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/cpuoptions"
)

func TestCPUOptions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CPUOptions")
}

var (
	// m5.4xlarge: 8 cores x 2 threads, both thread counts valid
	hyperthreaded = &ec2types.VCpuInfo{
		DefaultVCpus:          lo.ToPtr(int32(16)),
		DefaultCores:          lo.ToPtr(int32(8)),
		DefaultThreadsPerCore: lo.ToPtr(int32(2)),
		ValidCores:            []int32{2, 4, 6, 8},
		ValidThreadsPerCore:   []int32{1, 2},
	}
	// c6g.large: 2 cores x 1 thread, only 1 thread per core valid
	singleThreaded = &ec2types.VCpuInfo{
		DefaultVCpus:          lo.ToPtr(int32(2)),
		DefaultCores:          lo.ToPtr(int32(2)),
		DefaultThreadsPerCore: lo.ToPtr(int32(1)),
		ValidCores:            []int32{1, 2},
		ValidThreadsPerCore:   []int32{1},
	}
	// m5.metal: 48 cores x 2 threads, CpuOptions not supported so EC2 lists no valid values
	bareMetal = &ec2types.VCpuInfo{
		DefaultVCpus:          lo.ToPtr(int32(96)),
		DefaultCores:          lo.ToPtr(int32(48)),
		DefaultThreadsPerCore: lo.ToPtr(int32(2)),
	}
)

var _ = Describe("CPUOptions", func() {
	DescribeTable("Resolve",
		func(vcpuInfo *ec2types.VCpuInfo, cpuOptions *v1.CPUOptions, expected cpuoptions.Topology) {
			Expect(cpuoptions.Resolve(vcpuInfo, cpuOptions)).To(Equal(expected))
		},
		Entry("nil CPUOptions keeps the default layout", hyperthreaded, nil, cpuoptions.Topology{VCPUs: 16}),
		Entry("CPUOptions without threadsPerCore keeps the default layout", hyperthreaded, &v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled")}, cpuoptions.Topology{VCPUs: 16}),
		Entry("threadsPerCore=1 halves the vCPUs of a hyperthreaded instance type", hyperthreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, cpuoptions.Topology{VCPUs: 8, CoreCount: 8, ThreadsPerCore: 1}),
		Entry("threadsPerCore=2 matches the default of a hyperthreaded instance type", hyperthreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(2))}, cpuoptions.Topology{VCPUs: 16}),
		Entry("threadsPerCore=1 matches the default of a single-threaded instance type", singleThreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, cpuoptions.Topology{VCPUs: 2}),
		Entry("threadsPerCore=2 on a single-threaded instance type is explicit", singleThreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(2))}, cpuoptions.Topology{VCPUs: 4, CoreCount: 2, ThreadsPerCore: 2}),
		Entry("threadsPerCore=1 on bare metal is explicit", bareMetal, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, cpuoptions.Topology{VCPUs: 48, CoreCount: 48, ThreadsPerCore: 1}),
	)
	DescribeTable("Supported",
		func(vcpuInfo *ec2types.VCpuInfo, cpuOptions *v1.CPUOptions, expected bool) {
			Expect(cpuoptions.Supported(vcpuInfo, cpuOptions)).To(Equal(expected))
		},
		Entry("nil CPUOptions is always supported", bareMetal, nil, true),
		Entry("the default layout is always supported", singleThreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, true),
		Entry("an explicit layout is supported when EC2 lists both values as valid", hyperthreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, true),
		Entry("an explicit layout is not supported when the thread count is not valid", singleThreaded, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(2))}, false),
		Entry("an explicit layout is not supported when the instance type has no valid values", bareMetal, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, false),
		Entry("an explicit layout is not supported when the default core count is not valid", &ec2types.VCpuInfo{
			DefaultVCpus:          lo.ToPtr(int32(4)),
			DefaultCores:          lo.ToPtr(int32(2)),
			DefaultThreadsPerCore: lo.ToPtr(int32(2)),
			ValidCores:            []int32{1},
			ValidThreadsPerCore:   []int32{1, 2},
		}, &v1.CPUOptions{ThreadsPerCore: lo.ToPtr(int32(1))}, false),
	)
	It("should report an explicit topology only when a core count is set", func() {
		Expect(cpuoptions.Topology{VCPUs: 2}.Explicit()).To(BeFalse())
		Expect(cpuoptions.Topology{VCPUs: 2, CoreCount: 2, ThreadsPerCore: 1}.Explicit()).To(BeTrue())
	})
})
