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

package instancetype

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
)

func TestLabelInstanceCPUBurstable(t *testing.T) {
	amiFamily := amifamily.GetAMIFamily("AL2023", &amifamily.Options{})

	// Burstable instance (e.g. t3.medium): BurstablePerformanceSupported = true
	burstableInfo := ec2types.InstanceTypeInfo{
		InstanceType:                  "t3.medium",
		BurstablePerformanceSupported: aws.Bool(true),
		VCpuInfo:                      &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(2)},
		MemoryInfo:                    &ec2types.MemoryInfo{SizeInMiB: aws.Int64(4096)},
		Hypervisor:                    ec2types.InstanceTypeHypervisorNitro,
		ProcessorInfo: &ec2types.ProcessorInfo{
			SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
		},
		NetworkInfo: &ec2types.NetworkInfo{
			EncryptionInTransitSupported: aws.Bool(false),
		},
	}

	// Standard instance (e.g. m5.large): BurstablePerformanceSupported = false
	standardInfo := ec2types.InstanceTypeInfo{
		InstanceType:                  "m5.large",
		BurstablePerformanceSupported: aws.Bool(false),
		VCpuInfo:                      &ec2types.VCpuInfo{DefaultVCpus: aws.Int32(2)},
		MemoryInfo:                    &ec2types.MemoryInfo{SizeInMiB: aws.Int64(8192)},
		Hypervisor:                    ec2types.InstanceTypeHypervisorNitro,
		ProcessorInfo: &ec2types.ProcessorInfo{
			SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
		},
		NetworkInfo: &ec2types.NetworkInfo{
			EncryptionInTransitSupported: aws.Bool(true),
		},
	}

	// Verify burstable instance gets label value "true"
	burstableReqs := computeRequirements(burstableInfo, "us-east-1", nil, nil, amiFamily, nil)
	burstableVals := burstableReqs.Get(v1.LabelInstanceCPUBurstable).Values()
	if !burstableVals.Has("true") {
		t.Errorf("expected burstable instance (t3.medium) to have %s=\"true\", got %v", v1.LabelInstanceCPUBurstable, burstableVals)
	}

	// Verify standard instance gets label value "false"
	standardReqs := computeRequirements(standardInfo, "us-east-1", nil, nil, amiFamily, nil)
	standardVals := standardReqs.Get(v1.LabelInstanceCPUBurstable).Values()
	if !standardVals.Has("false") {
		t.Errorf("expected standard instance (m5.large) to have %s=\"false\", got %v", v1.LabelInstanceCPUBurstable, standardVals)
	}
}
