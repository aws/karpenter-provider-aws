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

// Package cpuoptions resolves the vCPU layout an instance type launches with under an EC2NodeClass'
// spec.cpuOptions. It is the single source of truth for that layout: the instance type provider uses it
// to compute capacity, overhead and labels, the compatibility check uses it to exclude instance types that
// can't satisfy the request, and the launch template resolver uses it to fill in EC2's CpuOptions. Keeping
// them on one function is what guarantees the scheduler and the launched instance agree on the vCPU count.
package cpuoptions

import (
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

// Topology is the vCPU layout an instance type launches with.
type Topology struct {
	// VCPUs is the number of vCPUs the instance boots with, which is what kubelet reports as CPU capacity.
	VCPUs int32
	// CoreCount and ThreadsPerCore are the values to request through the launch template's CpuOptions. EC2
	// requires both to be set together, so they are either both set or both zero. Both are zero when the
	// instance type's default layout already satisfies the NodeClass, in which case no CpuOptions are needed.
	CoreCount      int32
	ThreadsPerCore int32
}

// Explicit reports whether the layout has to be requested through the launch template's CpuOptions.
func (t Topology) Explicit() bool {
	return t.CoreCount != 0
}

// Resolve returns the layout the instance type launches with under cpuOptions.
//
// With cpuOptions.threadsPerCore unset, or equal to the instance type's default threads per core, the
// instance launches with its default layout and vCPU count. Otherwise the instance keeps its default core
// count and runs the requested number of threads on each, so the vCPU count becomes cores * threadsPerCore.
// Resolve does not check whether the instance type supports the request; see Supported.
func Resolve(vcpuInfo *ec2types.VCpuInfo, cpuOptions *v1.CPUOptions) Topology {
	defaultTopology := Topology{VCPUs: lo.FromPtr(vcpuInfo.DefaultVCpus)}
	if cpuOptions == nil || cpuOptions.ThreadsPerCore == nil {
		return defaultTopology
	}
	threadsPerCore := lo.FromPtr(cpuOptions.ThreadsPerCore)
	if lo.FromPtr(vcpuInfo.DefaultThreadsPerCore) == threadsPerCore {
		return defaultTopology
	}
	coreCount := lo.FromPtr(vcpuInfo.DefaultCores)
	return Topology{
		VCPUs:          coreCount * threadsPerCore,
		CoreCount:      coreCount,
		ThreadsPerCore: threadsPerCore,
	}
}

// Supported reports whether the instance type can launch with the layout Resolve returns for cpuOptions.
// An instance type that launches with its default layout always can. One that needs explicit CpuOptions
// can only when EC2 lists both the requested threads per core and the default core count as valid values
// for it; instance types that don't support CpuOptions at all, such as bare metal, list neither.
func Supported(vcpuInfo *ec2types.VCpuInfo, cpuOptions *v1.CPUOptions) bool {
	topology := Resolve(vcpuInfo, cpuOptions)
	if !topology.Explicit() {
		return true
	}
	return lo.Contains(vcpuInfo.ValidThreadsPerCore, topology.ThreadsPerCore) && lo.Contains(vcpuInfo.ValidCores, topology.CoreCount)
}
