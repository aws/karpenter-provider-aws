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

package aws

import (
	"fmt"

	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/vpc"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/ptr"
)

// EC2VMAvailableMemoryFactor assumes the EC2 VM will consume <7.25% of the memory of a given machine
const EC2VMAvailableMemoryFactor = .925

type InstanceType struct {
	ec2.InstanceTypeInfo
	AvailableOfferings []cloudprovider.Offering
	MaxPods            *int32
}

func (i *InstanceType) Name() string {
	return aws.StringValue(i.InstanceType)
}

func (i *InstanceType) Offerings() []cloudprovider.Offering {
	return i.AvailableOfferings
}

func (i *InstanceType) OperatingSystems() sets.String {
	return sets.NewString("linux")
}

func (i *InstanceType) Architecture() string {
	for _, architecture := range i.ProcessorInfo.SupportedArchitectures {
		if value, ok := v1alpha1.AWSToKubeArchitectures[aws.StringValue(architecture)]; ok {
			return value
		}
	}
	return aws.StringValueSlice(i.ProcessorInfo.SupportedArchitectures)[0] // Unrecognized, return the first architecture value for error printing
}

func (i *InstanceType) CPU() *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*i.VCpuInfo.DefaultVCpus))
}

func (i *InstanceType) Memory() *resource.Quantity {
	return resources.Quantity(
		fmt.Sprintf("%dMi", int32(
			float64(*i.MemoryInfo.SizeInMiB)*EC2VMAvailableMemoryFactor,
		)),
	)
}

func (i *InstanceType) Pods() *resource.Quantity {
	if i.MaxPods != nil {
		return resources.Quantity(fmt.Sprint(ptr.Int32Value(i.MaxPods)))
	}
	return resources.Quantity(fmt.Sprint(i.eniLimitedPods()))
}

func (i *InstanceType) AWSPodENI() *resource.Quantity {
	// https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html#supported-instance-types
	limits, ok := vpc.Limits[aws.StringValue(i.InstanceType)]
	if ok && limits.IsTrunkingCompatible {
		return resources.Quantity(fmt.Sprint(limits.BranchInterface))
	}
	return resources.Quantity("0")
}

func (i *InstanceType) NvidiaGPUs() *resource.Quantity {
	count := int64(0)
	if i.GpuInfo != nil {
		for _, gpu := range i.GpuInfo.Gpus {
			if *i.GpuInfo.Gpus[0].Manufacturer == "NVIDIA" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func (i *InstanceType) AMDGPUs() *resource.Quantity {
	count := int64(0)
	if i.GpuInfo != nil {
		for _, gpu := range i.GpuInfo.Gpus {
			if *i.GpuInfo.Gpus[0].Manufacturer == "AMD" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func (i *InstanceType) AWSNeurons() *resource.Quantity {
	count := int64(0)
	if i.InferenceAcceleratorInfo != nil {
		for _, accelerator := range i.InferenceAcceleratorInfo.Accelerators {
			count += *accelerator.Count
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

// Overhead computes overhead for https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#node-allocatable
// using calculations copied from https://github.com/bottlerocket-os/bottlerocket#kubernetes-settings.
// While this doesn't calculate the correct overhead for non-ENI-limited nodes, we're using this approach until further
// analysis can be performed
func (i *InstanceType) Overhead() v1.ResourceList {
	overhead := v1.ResourceList{
		v1.ResourceCPU: *resource.NewMilliQuantity(
			100, // system-reserved
			resource.DecimalSI),
		v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi",
			// kube-reserved
			((11*i.eniLimitedPods())+255)+
				// system-reserved
				100+
				// eviction threshold https://github.com/kubernetes/kubernetes/blob/ea0764452222146c47ec826977f49d7001b0ea8c/pkg/kubelet/apis/config/v1beta1/defaults_linux.go#L23
				100,
		)),
	}
	// kube-reserved Computed from
	// https://github.com/bottlerocket-os/bottlerocket/pull/1388/files#diff-bba9e4e3e46203be2b12f22e0d654ebd270f0b478dd34f40c31d7aa695620f2fR611
	for _, cpuRange := range []struct {
		start      int64
		end        int64
		percentage float64
	}{
		{start: 0, end: 1000, percentage: 0.06},
		{start: 1000, end: 2000, percentage: 0.01},
		{start: 2000, end: 4000, percentage: 0.005},
		{start: 4000, end: 1 << 31, percentage: 0.0025},
	} {
		if cpu := i.CPU().MilliValue(); cpu >= cpuRange.start {
			r := float64(cpuRange.end - cpuRange.start)
			if cpu < cpuRange.end {
				r = float64(cpu - cpuRange.start)
			}
			cpuOverhead := overhead[v1.ResourceCPU]
			cpuOverhead.Add(*resource.NewMilliQuantity(int64(r*cpuRange.percentage), resource.DecimalSI))
			overhead[v1.ResourceCPU] = cpuOverhead
		}
	}
	return overhead
}

// The number of pods per node is calculated using the formula:
// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
func (i *InstanceType) eniLimitedPods() int64 {
	return *i.NetworkInfo.MaximumNetworkInterfaces*(*i.NetworkInfo.Ipv4AddressesPerInterface-1) + 2
}

func (i *InstanceType) IsBareMetal() bool {
	return aws.BoolValue(i.BareMetal)
}
