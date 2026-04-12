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
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

func TestCpuOptions_Nil(t *testing.T) {
	if cpuOptions(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestCpuOptions_AllFieldsNil(t *testing.T) {
	if cpuOptions(&v1.CPUOptions{}) != nil {
		t.Fatal("expected nil for empty CPUOptions")
	}
}

func TestCpuOptions_NestedVirtualizationEnabled(t *testing.T) {
	result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled")})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.NestedVirtualization != ec2types.NestedVirtualizationSpecification("enabled") {
		t.Fatalf("expected 'enabled', got %q", result.NestedVirtualization)
	}
}

func TestCpuOptions_NestedVirtualizationDisabled(t *testing.T) {
	result := cpuOptions(&v1.CPUOptions{NestedVirtualization: lo.ToPtr("disabled")})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.NestedVirtualization != ec2types.NestedVirtualizationSpecification("disabled") {
		t.Fatalf("expected 'disabled', got %q", result.NestedVirtualization)
	}
}
