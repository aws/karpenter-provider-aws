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
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/api/resource"
)

// InstanceTypeVars holds the variables available to CEL expressions for kubelet configuration.
type InstanceTypeVars struct {
	VCPUs        int64
	MemoryMiB    int64
	DefaultENIs  int64
	IPsPerENI    int64
	MaxPods      int64
	InstanceType string
}

var (
	envOnce sync.Once
	envInst *cel.Env
	envErr  error
)

// Environment returns the shared CEL environment configured with instance type variables.
// The environment is created once and cached for the lifetime of the process.
func Environment() (*cel.Env, error) {
	envOnce.Do(func() {
		envInst, envErr = cel.NewEnv(
			cel.Variable("vcpus", cel.IntType),
			cel.Variable("memory_mib", cel.IntType),
			cel.Variable("default_enis", cel.IntType),
			cel.Variable("ips_per_eni", cel.IntType),
			cel.Variable("max_pods", cel.IntType),
			cel.Variable("instance_type", cel.StringType),
			cel.Function("max",
				cel.Overload("max_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
					cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
						l := lhs.(types.Int)
						r := rhs.(types.Int)
						if l > r {
							return l
						}
						return r
					}),
				),
				cel.Overload("max_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
					cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
						l := lhs.(types.Double)
						r := rhs.(types.Double)
						if l > r {
							return l
						}
						return r
					}),
				),
			),
			cel.Function("min",
				cel.Overload("min_int_int", []*cel.Type{cel.IntType, cel.IntType}, cel.IntType,
					cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
						l := lhs.(types.Int)
						r := rhs.(types.Int)
						if l < r {
							return l
						}
						return r
					}),
				),
				cel.Overload("min_double_double", []*cel.Type{cel.DoubleType, cel.DoubleType}, cel.DoubleType,
					cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
						l := lhs.(types.Double)
						r := rhs.(types.Double)
						if l < r {
							return l
						}
						return r
					}),
				),
			),
		)
	})
	return envInst, envErr
}

// CompiledExpression is a pre-compiled CEL program ready for evaluation.
type CompiledExpression struct {
	program cel.Program
}

// Compile parses and type-checks a CEL expression against the kubelet expression environment.
func Compile(expression string) (*CompiledExpression, error) {
	env, err := Environment()
	if err != nil {
		return nil, fmt.Errorf("creating CEL environment: %w", err)
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compiling expression %q: %w", expression, issues.Err())
	}
	if ast.OutputType() != cel.IntType && ast.OutputType() != cel.DoubleType {
		return nil, fmt.Errorf("expression %q must return int or double, got %v", expression, ast.OutputType())
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("creating program for expression %q: %w", expression, err)
	}
	return &CompiledExpression{program: prg}, nil
}

// Evaluate runs the compiled expression with the given instance type variables and returns the integer result.
func (c *CompiledExpression) Evaluate(vars InstanceTypeVars) (int64, error) {
	activation := map[string]any{
		"vcpus":         vars.VCPUs,
		"memory_mib":    vars.MemoryMiB,
		"default_enis":  vars.DefaultENIs,
		"ips_per_eni":   vars.IPsPerENI,
		"max_pods":      vars.MaxPods,
		"instance_type": vars.InstanceType,
	}
	out, _, err := c.program.Eval(activation)
	if err != nil {
		return 0, fmt.Errorf("evaluating expression: %w", err)
	}
	switch v := out.Value().(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("expression returned unexpected type %T", out.Value())
	}
}

// EvaluateExpression compiles and evaluates a CEL expression in one call.
// For repeated evaluations with the same expression, prefer Compile() followed by Evaluate().
func EvaluateExpression(expression string, vars InstanceTypeVars) (int64, error) {
	compiled, err := Compile(expression)
	if err != nil {
		return 0, err
	}
	return compiled.Evaluate(vars)
}

// ValidateExpression checks if a CEL expression compiles successfully without evaluating it.
func ValidateExpression(expression string) error {
	_, err := Compile(expression)
	return err
}

// ResolveResourceMap evaluates the CEL expressions in a kubelet resource map (kubeReserved or
// systemReserved). Values that already parse as valid Kubernetes resource quantities are passed
// through unchanged; values that don't are evaluated as CEL expressions and replaced with their
// integer result. Entries whose expression fails to evaluate or yields a negative value are
// dropped (and logged).
//
// varsFn is called at most once, and only when the map actually contains an expression, so callers
// can defer expensive variable construction. This is the single evaluation path shared by both the
// scheduler (reserved-capacity overhead) and the launch template resolver so that identical inputs
// always produce identical results.
func ResolveResourceMap(resourceMap map[string]string, varsFn func() InstanceTypeVars, log logr.Logger) map[string]string {
	if len(resourceMap) == 0 {
		return resourceMap
	}
	var vars InstanceTypeVars
	varsBuilt := false
	resolved := make(map[string]string, len(resourceMap))
	for k, v := range resourceMap {
		if _, err := resource.ParseQuantity(v); err == nil {
			resolved[k] = v
			continue
		}
		if !varsBuilt {
			vars = varsFn()
			varsBuilt = true
		}
		result, err := EvaluateExpression(v, vars)
		if err != nil {
			log.Error(err, "failed to evaluate kubelet resource expression", "key", k, "expression", v, "instanceType", vars.InstanceType)
			continue
		}
		if result < 0 {
			log.Error(fmt.Errorf("result %d is negative", result), "kubelet resource expression evaluated to an invalid value", "key", k, "expression", v, "instanceType", vars.InstanceType)
			continue
		}
		resolved[k] = fmt.Sprint(result)
	}
	return resolved
}
