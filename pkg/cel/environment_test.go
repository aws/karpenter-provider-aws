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

package cel

import (
	"testing"
)

func TestEvaluateExpression_ENIFormula(t *testing.T) {
	// m5.large: 3 ENIs, 10 IPs/ENI -> ((3-1) * (10-1)) + 2 = 20
	vars := InstanceTypeVars{
		VCPUs:       2,
		MemoryMiB:   8192,
		DefaultENIs: 3,
		IPsPerENI:   10,
		MaxPods:     20,
	}
	result, err := EvaluateExpression("((default_enis - 1) * (ips_per_eni - 1)) + 2", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Fatalf("expected 20, got %d", result)
	}
}

func TestEvaluateExpression_PrefixDelegation(t *testing.T) {
	// m5.large with prefix delegation: min(250, ((3-1) * (10-1)) * 16 + 2) = min(250, 290) = 250
	vars := InstanceTypeVars{
		VCPUs:       2,
		MemoryMiB:   8192,
		DefaultENIs: 3,
		IPsPerENI:   10,
		MaxPods:     20,
	}
	result, err := EvaluateExpression("min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 250 {
		t.Fatalf("expected 250, got %d", result)
	}
}

func TestEvaluateExpression_KubeReservedCPU(t *testing.T) {
	// 16 vCPUs: max(60, 16 * 30) * 1000000 = 480000000 (480m in nanocores)
	vars := InstanceTypeVars{
		VCPUs:       16,
		MemoryMiB:   65536,
		DefaultENIs: 8,
		IPsPerENI:   30,
		MaxPods:     58,
	}
	result, err := EvaluateExpression("max(60, vcpus * 30) * 1000000", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 480000000 {
		t.Fatalf("expected 480000000, got %d", result)
	}
}

func TestEvaluateExpression_KubeReservedMemory(t *testing.T) {
	// (11 * 58 + 255) * 1048576
	vars := InstanceTypeVars{
		VCPUs:       16,
		MemoryMiB:   65536,
		DefaultENIs: 8,
		IPsPerENI:   30,
		MaxPods:     58,
	}
	result, err := EvaluateExpression("(11 * max_pods + 255) * 1048576", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64((11*58 + 255) * 1048576)
	if result != expected {
		t.Fatalf("expected %d, got %d", expected, result)
	}
}

func TestEvaluateExpression_MinMax(t *testing.T) {
	vars := InstanceTypeVars{VCPUs: 4, MemoryMiB: 8192, DefaultENIs: 3, IPsPerENI: 10, MaxPods: 20}
	result, err := EvaluateExpression("min(110, max_pods)", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 20 {
		t.Fatalf("expected 20, got %d", result)
	}
}

func TestValidateExpression_Valid(t *testing.T) {
	if err := ValidateExpression("((default_enis - 1) * (ips_per_eni - 1)) + 2"); err != nil {
		t.Fatalf("expected valid expression, got: %v", err)
	}
}

func TestValidateExpression_InvalidSyntax(t *testing.T) {
	if err := ValidateExpression("((default_enis -"); err == nil {
		t.Fatal("expected error for invalid syntax")
	}
}

func TestValidateExpression_UndefinedVariable(t *testing.T) {
	if err := ValidateExpression("undefined_var + 1"); err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

func TestValidateExpression_WrongReturnType(t *testing.T) {
	if err := ValidateExpression("vcpus > 4"); err == nil {
		t.Fatal("expected error for boolean return type")
	}
}
