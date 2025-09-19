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
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
)

func TestMemory(t *testing.T) {
	tests := []struct {
		name                    string
		instanceTypeInfo        ec2types.InstanceTypeInfo
		vmMemoryOverheadPercent float64
		expectedMemoryBytes     int64
	}{
		{
			name: "x86_64 instance without CMA reservation",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(8192), // 8 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
				},
			},
			vmMemoryOverheadPercent: 0.075,      // 7.5%
			expectedMemoryBytes:     7945060352, // 8192 MiB - 7.5% overhead in bytes
		},
		{
			name: "ARM64 instance with CMA reservation",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(8192), // 8 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
				},
			},
			vmMemoryOverheadPercent: 0.075,      // 7.5%
			expectedMemoryBytes:     7883194368, // (8192 - 64) MiB - 7.5% overhead in bytes
		},
		{
			name: "ARM64 instance with zero VM overhead",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(4096), // 4 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
				},
			},
			vmMemoryOverheadPercent: 0.0,        // 0%
			expectedMemoryBytes:     4227858432, // (4096 - 64) MiB in bytes
		},
		{
			name: "x86_64 instance with zero VM overhead",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(4096), // 4 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
				},
			},
			vmMemoryOverheadPercent: 0.0,        // 0%
			expectedMemoryBytes:     4294967296, // 4096 MiB in bytes
		},
		{
			name: "ARM64 instance with high VM overhead",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(16384), // 16 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
				},
			},
			vmMemoryOverheadPercent: 0.15,        // 15%
			expectedMemoryBytes:     14545846272, // (16384 - 64) MiB - 15% overhead in bytes
		},
		{
			name: "x86_64 instance with high VM overhead",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(16384), // 16 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
				},
			},
			vmMemoryOverheadPercent: 0.15,        // 15%
			expectedMemoryBytes:     14602469376, // 16384 MiB - 15% overhead in bytes
		},
		{
			name: "ARM64 instance with small memory size",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(512), // 512 MiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
				},
			},
			vmMemoryOverheadPercent: 0.075,     // 7.5%
			expectedMemoryBytes:     434110464, // (512 - 64) MiB - 7.5% overhead in bytes
		},
		{
			name: "Empty processor info (edge case)",
			instanceTypeInfo: ec2types.InstanceTypeInfo{
				MemoryInfo: &ec2types.MemoryInfo{
					SizeInMiB: aws.Int64(2048), // 2 GiB
				},
				ProcessorInfo: &ec2types.ProcessorInfo{
					SupportedArchitectures: []ec2types.ArchitectureType{}, // Empty slice
				},
			},
			vmMemoryOverheadPercent: 0.075,      // 7.5%
			expectedMemoryBytes:     1986002944, // 2048 MiB - 7.5% overhead in bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with VM memory overhead percent
			opts := &options.Options{
				VMMemoryOverheadPercent: tt.vmMemoryOverheadPercent,
			}
			ctx := options.ToContext(context.Background(), opts)

			// Call the memory function
			result := memory(ctx, tt.instanceTypeInfo)

			// Verify the result (result.Value() returns bytes)
			if result.Value() != tt.expectedMemoryBytes {
				t.Errorf("memory() = %d, want %d", result.Value(), tt.expectedMemoryBytes)
			}

			// Verify the result is in MiB format
			if result.Format != resource.BinarySI {
				t.Errorf("memory() format = %v, want %v", result.Format, resource.BinarySI)
			}
		})
	}
}

func TestMemoryEdgeCases(t *testing.T) {
	t.Run("nil memory info", func(t *testing.T) {
		opts := &options.Options{VMMemoryOverheadPercent: 0.075}
		ctx := options.ToContext(context.Background(), opts)

		instanceTypeInfo := ec2types.InstanceTypeInfo{
			MemoryInfo: nil,
			ProcessorInfo: &ec2types.ProcessorInfo{
				SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
			},
		}

		// This should panic since we're dereferencing a nil pointer
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for nil MemoryInfo, but function didn't panic")
			}
		}()

		memory(ctx, instanceTypeInfo)
	})

	t.Run("nil processor info", func(t *testing.T) {
		opts := &options.Options{VMMemoryOverheadPercent: 0.075}
		ctx := options.ToContext(context.Background(), opts)

		instanceTypeInfo := ec2types.InstanceTypeInfo{
			MemoryInfo: &ec2types.MemoryInfo{
				SizeInMiB: aws.Int64(1024),
			},
			ProcessorInfo: nil,
		}

		// This should panic since we're dereferencing a nil pointer
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for nil ProcessorInfo, but function didn't panic")
			}
		}()

		memory(ctx, instanceTypeInfo)
	})
}

func TestMemoryCalculationAccuracy(t *testing.T) {
	t.Run("verify CMA reservation calculation", func(t *testing.T) {
		opts := &options.Options{VMMemoryOverheadPercent: 0.0} // No overhead for easier calculation
		ctx := options.ToContext(context.Background(), opts)

		// Test ARM64 instance
		arm64Info := ec2types.InstanceTypeInfo{
			MemoryInfo: &ec2types.MemoryInfo{
				SizeInMiB: aws.Int64(1024),
			},
			ProcessorInfo: &ec2types.ProcessorInfo{
				SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
			},
		}

		result := memory(ctx, arm64Info)
		expected := int64(1024-64) * 1024 * 1024 // Should subtract 64 MiB for CMA reservation, convert to bytes

		if result.Value() != expected {
			t.Errorf("ARM64 CMA reservation: got %d, want %d", result.Value(), expected)
		}

		// Test x86_64 instance
		x86Info := ec2types.InstanceTypeInfo{
			MemoryInfo: &ec2types.MemoryInfo{
				SizeInMiB: aws.Int64(1024),
			},
			ProcessorInfo: &ec2types.ProcessorInfo{
				SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
			},
		}

		result = memory(ctx, x86Info)
		expected = int64(1024) * 1024 * 1024 // Should not subtract anything for x86_64, convert to bytes

		if result.Value() != expected {
			t.Errorf("x86_64 no CMA reservation: got %d, want %d", result.Value(), expected)
		}
	})

	t.Run("verify VM overhead calculation", func(t *testing.T) {
		// Test with 10% overhead
		opts := &options.Options{VMMemoryOverheadPercent: 0.10}
		ctx := options.ToContext(context.Background(), opts)

		instanceTypeInfo := ec2types.InstanceTypeInfo{
			MemoryInfo: &ec2types.MemoryInfo{
				SizeInMiB: aws.Int64(1000), // 1000 MiB
			},
			ProcessorInfo: &ec2types.ProcessorInfo{
				SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
			},
		}

		result := memory(ctx, instanceTypeInfo)

		// Expected: 1000 MiB - 10% overhead in bytes
		// Base memory: 1000 MiB = 1000 * 1024 * 1024 bytes
		// VM overhead calculation: mem.Value() * 0.10 / 1024 / 1024
		// Since mem.Value() is already in bytes, dividing by 1024/1024 gives us the same value
		// So overhead = 1000 * 1024 * 1024 * 0.10 = 104,857,600 bytes
		// Final = 1,048,576,000 - 104,857,600 = 943,718,400 bytes
		expected := int64(float64(1000) * 1024 * 1024 * (1 - 0.10)) // 1000 MiB - 10% overhead

		if result.Value() != expected {
			t.Errorf("VM overhead calculation: got %d, want %d", result.Value(), expected)
		}
	})
}
