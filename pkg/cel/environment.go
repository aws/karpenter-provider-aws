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
	"math"
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

// Used to both declare the variables on the CEL environment and to build the
// activation map at evaluation time
var celVars = []struct {
	name    string
	celType *cel.Type
	get     func(InstanceTypeVars) any
}{
	{"vcpus", cel.IntType, func(v InstanceTypeVars) any { return v.VCPUs }},
	{"memory_mib", cel.IntType, func(v InstanceTypeVars) any { return v.MemoryMiB }},
	{"default_enis", cel.IntType, func(v InstanceTypeVars) any { return v.DefaultENIs }},
	{"ips_per_eni", cel.IntType, func(v InstanceTypeVars) any { return v.IPsPerENI }},
	{"max_pods", cel.IntType, func(v InstanceTypeVars) any { return v.MaxPods }},
	{"instance_type", cel.StringType, func(v InstanceTypeVars) any { return v.InstanceType }},
}

// CELEnvironment wraps a configured CEL environment together with a compilation cache. Callers construct
// one via NewEnvironment and inject it wherever kubelet expressions are compiled or evaluated, so the same
// environment (and cache) is shared across the scheduler, the launch template resolver, and validation.
type CELEnvironment struct {
	env *cel.Env
	// compiledCache memoizes successful compilations keyed by expression string.
	compiledCache sync.Map
}

// NewEnvironment builds a CELEnvironment configured with the kubelet expression variables and functions.
// Construction has no runtime-variable inputs (a fixed set of variables and functions, no I/O), so an error
// here indicates a programming error in this package rather than bad user input.
func NewEnvironment() (*CELEnvironment, error) {
	opts := make([]cel.EnvOption, 0, len(celVars)+2)
	for _, v := range celVars {
		opts = append(opts, cel.Variable(v.name, v.celType))
	}
	opts = append(opts,
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
			cel.Overload("max_int_double", []*cel.Type{cel.IntType, cel.DoubleType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := types.Double(lhs.(types.Int))
					r := rhs.(types.Double)
					if l > r {
						return l
					}
					return r
				}),
			),
			cel.Overload("max_double_int", []*cel.Type{cel.DoubleType, cel.IntType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := lhs.(types.Double)
					r := types.Double(rhs.(types.Int))
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
			cel.Overload("min_int_double", []*cel.Type{cel.IntType, cel.DoubleType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := types.Double(lhs.(types.Int))
					r := rhs.(types.Double)
					if l < r {
						return l
					}
					return r
				}),
			),
			cel.Overload("min_double_int", []*cel.Type{cel.DoubleType, cel.IntType}, cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					l := lhs.(types.Double)
					r := types.Double(rhs.(types.Int))
					if l < r {
						return l
					}
					return r
				}),
			),
		),
	)
	e, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, fmt.Errorf("building CEL environment: %w", err)
	}
	return &CELEnvironment{env: e}, nil
}

// CompiledExpression is a pre-compiled CEL program ready for evaluation.
type CompiledExpression struct {
	program cel.Program
}

// compileCached returns a cached CompiledExpression for the expression, compiling and caching it on the
// first request. Only successful compilations are cached; failures are returned without being stored so a
// later corrected expression (or a transient issue) isn't pinned to its error.
func (c *CELEnvironment) compileCached(expression string) (*CompiledExpression, error) {
	if cached, ok := c.compiledCache.Load(expression); ok {
		return cached.(*CompiledExpression), nil
	}
	compiled, err := c.Compile(expression)
	if err != nil {
		return nil, err
	}
	c.compiledCache.Store(expression, compiled)
	return compiled, nil
}

// Compile parses and type-checks a CEL expression against the kubelet expression environment.
func (c *CELEnvironment) Compile(expression string) (*CompiledExpression, error) {
	ast, issues := c.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compiling expression %q: %w", expression, issues.Err())
	}
	if ast.OutputType() != cel.IntType && ast.OutputType() != cel.DoubleType {
		return nil, fmt.Errorf("expression %q must return int or double, got %v", expression, ast.OutputType())
	}
	prg, err := c.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("creating program for expression %q: %w", expression, err)
	}
	return &CompiledExpression{program: prg}, nil
}

// EvaluateExpression compiles (via the compilation cache) and evaluates a CEL expression against the
// given instance type variables, returning the integer result. Repeated calls with the same expression
// reuse the cached compiled program.
func (c *CELEnvironment) EvaluateExpression(expression string, vars InstanceTypeVars) (int64, error) {
	compiled, err := c.compileCached(expression)
	if err != nil {
		return 0, err
	}
	activation := make(map[string]any, len(celVars))
	for _, cv := range celVars {
		activation[cv.name] = cv.get(vars)
	}
	out, _, err := compiled.program.Eval(activation)
	if err != nil {
		return 0, fmt.Errorf("evaluating expression: %w", err)
	}
	switch v := out.Value().(type) {
	case int64:
		return v, nil
	case float64:
		// Reject non-finite doubles (+Inf, -Inf, NaN) — e.g. from double division by zero.
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return 0, fmt.Errorf("expression returned non-finite value %v", v)
		}
		return int64(v), nil
	default:
		return 0, fmt.Errorf("expression returned unexpected type %T", out.Value())
	}
}

// ValidateExpression checks if a CEL expression compiles successfully without evaluating it.
func (c *CELEnvironment) ValidateExpression(expression string) error {
	_, err := c.compileCached(expression)
	return err
}

// ResolveResourceMap evaluates the CEL expressions in a kubelet resource map (kubeReserved or
// systemReserved). Values that already parse as valid Kubernetes resource quantities are passed
// through unchanged; values that don't are evaluated as CEL expressions and replaced with their
// integer result. Entries whose expression fails to evaluate or yields a negative value are
// dropped (and logged).
//
// varsFn is called at most once, and only when the map actually contains an expression, so callers
// can defer expensive variable construction. If varsFn returns an error (e.g. the instance type's
// inputs couldn't be resolved), it is returned to the caller so the failure can be surfaced up the
// call chain. This is the single evaluation path shared by both the scheduler (reserved-capacity overhead) 
// and the launch template resolver so that identical inputs always produce identical results.
func (c *CELEnvironment) ResolveResourceMap(resourceMap map[string]string, varsFn func() (InstanceTypeVars, error), log logr.Logger) (map[string]string, error) {
	if len(resourceMap) == 0 {
		return resourceMap, nil
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
			var err error
			if vars, err = varsFn(); err != nil {
				return nil, err
			}
			varsBuilt = true
		}
		result, err := c.EvaluateExpression(v, vars)
		if err != nil {
			log.Error(err, "failed to evaluate kubelet resource expression", "key", k, "expression", v, "instanceType", vars.InstanceType,
				"vcpus", vars.VCPUs, "memory_mib", vars.MemoryMiB, "default_enis", vars.DefaultENIs, "ips_per_eni", vars.IPsPerENI, "max_pods", vars.MaxPods)
			continue
		}
		if result < 0 {
			log.Error(fmt.Errorf("result %d is negative", result), "kubelet resource expression evaluated to an invalid value", "key", k, "expression", v, "instanceType", vars.InstanceType)
			continue
		}
		resolved[k] = fmt.Sprint(result)
	}
	return resolved, nil
}
