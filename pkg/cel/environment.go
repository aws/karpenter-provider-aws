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
	"time"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/patrickmn/go-cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	compiledExpressionTTL             = 12 * time.Hour
	compiledExpressionCleanupInterval = 1 * time.Hour

	// cpuRoundingMilliCores and memoryRoundingMiB are the granularities that cpu and memory expression
	// results are rounded up to.
	//
	// These are set to the granularity users already write these values at -- 10m of CPU and 16Mi of
	// memory -- so the adjustment stays within the noise of a hand-written reservation: the worst case is
	// +9m of CPU and +15Mi of memory, and the relative cost shrinks as the reservation grows.
	//
	// Rounding is always up, so a reservation is never smaller than the expression asked for.
	// Under-reserving risks node instability, whereas over-reserving by these amounts does not. Note that
	// how much rounding actually collapses depends on how far apart an expression's results are: formulas
	// whose results step by more than the granularity (e.g. "vcpus * 30", which steps by 30m) are
	// unaffected, while ones with finer steps (e.g. "70 + vcpus") collapse substantially.
	cpuRoundingMilliCores = 10
	memoryRoundingMiB     = 16
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
	// compiledCache memoizes successful compilations keyed by expression string. It uses a sliding-window TTL
	compiledCache *cache.Cache
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
	return &CELEnvironment{
		env:           e,
		compiledCache: cache.New(compiledExpressionTTL, compiledExpressionCleanupInterval),
	}, nil
}

// CompiledExpression is a pre-compiled CEL program ready for evaluation.
type CompiledExpression struct {
	program cel.Program
}

// compileCached returns a cached CompiledExpression for the expression, compiling and caching it on the
// first request. Only successful compilations are cached; failures are returned without being stored so a
// later corrected expression (or a transient issue) isn't pinned to its error.
func (c *CELEnvironment) compileCached(expression string) (*CompiledExpression, error) {
	if cached, ok := c.compiledCache.Get(expression); ok {
		// Refresh the TTL so a live expression isn't evicted out from under active use.
		c.compiledCache.Set(expression, cached, cache.DefaultExpiration)
		return cached.(*CompiledExpression), nil
	}
	compiled, err := c.Compile(expression)
	if err != nil {
		return nil, err
	}
	c.compiledCache.Set(expression, compiled, cache.DefaultExpiration)
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
		resolved[k] = formatResourceResult(k, result)
	}
	return resolved, nil
}

// formatResourceResult renders an expression's integer result as a Kubernetes resource quantity, attaching
// the unit suffix implied by the resource key. Expressions are written in the same units users already use
// for the static quantities in this field, so a formula reads like the value it replaces.
//
// cpu is millicores ("480m"): without the suffix, resource.ParseQuantity reads a bare integer as whole
// cores, so "max(60, vcpus * 30)" would reserve 480 *cores*, and a sub-core reservation would be
// inexpressible (the fractional value needed would truncate to 0 on the double -> int64 conversion in
// EvaluateExpression). Results are rounded up to cpuRoundingMilliCores.
//
// memory is mebibytes ("630Mi"), matching both the Mi quantities users write for this field and
// memory_mib's own unit, so that "memory_mib / 100" means 1% of node memory with no scale factor. Results
// are rounded up to memoryRoundingMiB, which subsumes the sub-MiB truncation that the double -> int64
// conversion already performs.
//
// ephemeral-storage is gibibytes ("3Gi"), the unit its static quantities and Karpenter's own 1Gi default
// use. Note that this makes GiB the granularity floor: a sub-GiB reservation is not expressible, which
// suits the whole-GiB values this field takes in practice but is coarser than memory's. It is not rounded
// further, since GiB is already coarse enough that any additional bucket would multiply the reservation
// rather than nudge it.
//
// pid is a unitless process count and is emitted bare. It isn't rounded: a process count has no natural
// granularity to round to, and unlike cpu/memory it doesn't consume a divisible node resource.
func formatResourceResult(key string, result int64) string {
	switch key {
	case string(corev1.ResourceCPU):
		return fmt.Sprintf("%dm", roundUpTo(result, cpuRoundingMilliCores))
	case string(corev1.ResourceMemory):
		return fmt.Sprintf("%dMi", roundUpTo(result, memoryRoundingMiB))
	case string(corev1.ResourceEphemeralStorage):
		return fmt.Sprintf("%dGi", result)
	default:
		return fmt.Sprint(result)
	}
}

// roundUpTo rounds value up to the next multiple of granularity, leaving exact multiples (including zero)
// untouched. Values within granularity of math.MaxInt64 are returned unrounded rather than overflowed into
// a negative quantity; such a value is nonsensical as a reservation either way, but it must not silently
// become negative, since ResolveResourceMap has already passed its non-negative check by this point.
func roundUpTo(value, granularity int64) int64 {
	if granularity <= 1 || value <= 0 || value%granularity == 0 {
		return value
	}
	if value > math.MaxInt64-granularity {
		return value
	}
	return ((value / granularity) + 1) * granularity
}
