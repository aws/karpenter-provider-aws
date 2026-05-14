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
)

func TestAdjustedCPU(t *testing.T) {
	tests := []struct {
		name             string
		instanceType     string
		defaultVCPUs     int32
		defaultCores     int32
		defaultThreads   int32
		cpuOptions       *v1.CPUOptions
		expectedCPUs     int64
		description      string
	}{
		{
			name:           "No cpuOptions - return default vCPUs",
			instanceType:   "r5a.24xlarge",
			defaultVCPUs:   96,
			defaultCores:   48,
			defaultThreads: 2,
			cpuOptions:     nil,
			expectedCPUs:   96,
			description:    "When cpuOptions is nil, should return default vCPUs",
		},
		{
			name:           "ThreadsPerCore=1 (hyperthreading disabled)",
			instanceType:   "r5a.24xlarge",
			defaultVCPUs:   96,
			defaultCores:   48,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				ThreadsPerCore: aws.Int64(1),
			},
			expectedCPUs: 48,
			description:  "48 cores * 1 thread = 48 vCPUs (HT disabled)",
		},
		{
			name:           "ThreadsPerCore=2 (hyperthreading enabled)",
			instanceType:   "r5a.24xlarge",
			defaultVCPUs:   96,
			defaultCores:   48,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				ThreadsPerCore: aws.Int64(2),
			},
			expectedCPUs: 96,
			description:  "48 cores * 2 threads = 96 vCPUs (HT enabled)",
		},
		{
			name:           "CoreCount only specified",
			instanceType:   "r5a.24xlarge",
			defaultVCPUs:   96,
			defaultCores:   48,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				CoreCount: aws.Int64(24),
			},
			expectedCPUs: 48,
			description:  "24 cores * 2 threads (default) = 48 vCPUs",
		},
		{
			name:           "Both CoreCount and ThreadsPerCore specified",
			instanceType:   "r5a.24xlarge",
			defaultVCPUs:   96,
			defaultCores:   48,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				CoreCount:      aws.Int64(24),
				ThreadsPerCore: aws.Int64(1),
			},
			expectedCPUs: 24,
			description:  "24 cores * 1 thread = 24 vCPUs",
		},
		{
			name:           "m5.large with HT disabled",
			instanceType:   "m5.large",
			defaultVCPUs:   2,
			defaultCores:   1,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				ThreadsPerCore: aws.Int64(1),
			},
			expectedCPUs: 1,
			description:  "1 core * 1 thread = 1 vCPU",
		},
		{
			name:           "c5.18xlarge with HT disabled",
			instanceType:   "c5.18xlarge",
			defaultVCPUs:   72,
			defaultCores:   36,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				ThreadsPerCore: aws.Int64(1),
			},
			expectedCPUs: 36,
			description:  "36 cores * 1 thread = 36 vCPUs",
		},
		{
			name:           "r6i.32xlarge with custom core count and HT disabled",
			instanceType:   "r6i.32xlarge",
			defaultVCPUs:   128,
			defaultCores:   64,
			defaultThreads: 2,
			cpuOptions: &v1.CPUOptions{
				CoreCount:      aws.Int64(32),
				ThreadsPerCore: aws.Int64(1),
			},
			expectedCPUs: 32,
			description:  "32 cores (custom) * 1 thread = 32 vCPUs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ec2types.InstanceTypeInfo{
				InstanceType: ec2types.InstanceType(tt.instanceType),
				VCpuInfo: &ec2types.VCpuInfo{
					DefaultVCpus:          aws.Int32(tt.defaultVCPUs),
					DefaultCores:          aws.Int32(tt.defaultCores),
					DefaultThreadsPerCore: aws.Int32(tt.defaultThreads),
				},
			}

			result := adjustedCPU(info, tt.cpuOptions)

			if result.Value() != tt.expectedCPUs {
				t.Errorf("%s: expected %d CPUs, got %d CPUs. %s",
					tt.name, tt.expectedCPUs, result.Value(), tt.description)
			}
		})
	}
}

// TestAdjustedCPUConsistency ensures that adjustedCPU with nil cpuOptions returns the same as cpu()
func TestAdjustedCPUConsistency(t *testing.T) {
	info := ec2types.InstanceTypeInfo{
		InstanceType: "m5.xlarge",
		VCpuInfo: &ec2types.VCpuInfo{
			DefaultVCpus:          aws.Int32(4),
			DefaultCores:          aws.Int32(2),
			DefaultThreadsPerCore: aws.Int32(2),
		},
	}

	resultAdjusted := adjustedCPU(info, nil)
	resultCPU := cpu(info)

	if resultAdjusted.Value() != resultCPU.Value() {
		t.Errorf("adjustedCPU(info, nil) should equal cpu(info), got %d vs %d",
			resultAdjusted.Value(), resultCPU.Value())
	}
}
