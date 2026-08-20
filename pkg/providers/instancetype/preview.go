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
	"fmt"
	"sort"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
)

// PreviewedExpression is the outcome of evaluating a single kubelet CEL expression for one instance type.
// Applied reports whether the controller would actually use Value: an expression that fails to evaluate,
// or that produces an out-of-range result, is dropped rather than applied, and Err explains why.
type PreviewedExpression struct {
	// Field is the kubelet configuration field the expression came from, e.g. "maxPods" or "kubeReserved[cpu]".
	Field string
	// Expression is the user-supplied CEL expression, verbatim.
	Expression string
	// Value is the final value the controller would apply, already formatted as the field's
	// resource quantity (e.g. "480m", "630Mi") for the reserved maps, or a bare count for maxPods.
	Value string
	// Applied is false when the expression was dropped, in which case Err is non-nil.
	Applied bool
	// Err is the reason the expression was dropped, or nil when Applied is true.
	Err error
}

// KubeletExpressionPreview is a full per-instance-type preview of a kubelet configuration's CEL expressions.
type KubeletExpressionPreview struct {
	InstanceType string
	// MaxPodsVars holds the variables the maxPods expression is evaluated against. Its MaxPods field is the
	// AMI-family default for this instance type, since a maxPods expression cannot reference its own result.
	MaxPodsVars kubeletcel.InstanceTypeVars
	// ReservedVars holds the variables kubeReserved and systemReserved expressions are evaluated against.
	// Its MaxPods field reflects the *resolved* maxPods, so max_pods means something different here than it
	// does in MaxPodsVars whenever maxPods is itself set. Zero-valued when no reserved expressions exist,
	// since the controller skips building it in that case.
	ReservedVars kubeletcel.InstanceTypeVars
	// ReservedVarsBuilt reports whether ReservedVars was populated.
	ReservedVarsBuilt bool
	// Expressions holds one entry per CEL expression found in the kubelet configuration. Static values are
	// not included, since they are passed through without evaluation.
	Expressions []PreviewedExpression
}

// PreviewKubeletExpressions evaluates every CEL expression in kc against a single instance type and reports
// what the controller would apply, without mutating kc or touching a cluster. It mirrors the ordering in
// evaluateKubeletExpressions exactly — maxPods is evaluated against the default max_pods, then
// kubeReserved/systemReserved are evaluated against the resolved maxPods — so a preview cannot diverge from
// what a real launch would compute. It is the shared engine behind the `celtest` tool.
func PreviewKubeletExpressions(
	ctx context.Context,
	celEnv *kubeletcel.CELEnvironment,
	info ec2types.InstanceTypeInfo,
	kc *v1.KubeletConfiguration,
	amiFamily amifamily.AMIFamily,
	networkInterfaces []*v1.NetworkInterface,
) KubeletExpressionPreview {
	preview := KubeletExpressionPreview{InstanceType: string(info.InstanceType)}
	if kc == nil {
		return preview
	}
	// The maxPods expression evaluates against the default max_pods (it can't self-reference), matching
	// evaluateKubeletExpressions.
	preview.MaxPodsVars = buildCELVars(ctx, info, amiFamily, nil, kc.PodsPerCore, networkInterfaces)
	// resolveMaxPods evaluates the maxPods expression against the default max_pods and range-checks the
	// result, so it is both the value the reserved vars are built from and the maxPods field's own outcome.
	// It is the exact call Resolve makes, so a preview reports the identical failure text.
	resolvedMaxPods, maxPodsErr := resolveMaxPods(ctx, celEnv, info, kc.MaxPods, amiFamily, kc.PodsPerCore, networkInterfaces)
	if maxPodsExpr, ok := maxPodsExpression(kc); ok {
		p := PreviewedExpression{Field: "maxPods", Expression: maxPodsExpr}
		if maxPodsErr != nil {
			p.Err = maxPodsErr
		} else if resolvedMaxPods != nil {
			p.Value = formatCount(int64(*resolvedMaxPods))
			p.Applied = true
		}
		preview.Expressions = append(preview.Expressions, p)
	}
	if !kc.HasResourceExpressions() {
		return preview
	}
	// Reserved expressions see the resolved maxPods, not the default.
	preview.ReservedVars = buildCELVars(ctx, info, amiFamily, resolvedMaxPods, kc.PodsPerCore, networkInterfaces)
	preview.ReservedVarsBuilt = true
	for _, m := range []struct {
		field string
		m     map[string]string
	}{
		{"kubeReserved", kc.KubeReserved},
		{"systemReserved", kc.SystemReserved},
	} {
		preview.Expressions = append(preview.Expressions, previewResourceMap(ctx, celEnv, m.field, m.m, preview.ReservedVars)...)
	}
	return preview
}

// previewResourceMap evaluates the expressions in one reserved map, reusing ResolveResourceMap so the
// emitted values carry the same per-key unit suffix (and the same drop-on-failure semantics) that the
// controller produces. Static quantities are skipped, since they are never evaluated.
func previewResourceMap(ctx context.Context, celEnv *kubeletcel.CELEnvironment, field string, m map[string]string, vars kubeletcel.InstanceTypeVars) []PreviewedExpression {
	// A discarding logger keeps the dropped-entry diagnostics out of the tool's output; the reason is
	// recomputed below for display.
	ctx = log.IntoContext(ctx, logr.Discard())
	var out []PreviewedExpression
	for _, key := range sortedExpressionKeys(m) {
		expr := m[key]
		p := PreviewedExpression{Field: field + "[" + key + "]", Expression: expr}
		// Route the single entry through the real resolver so formatting and the negative/non-finite drop
		// rules are the production ones rather than a reimplementation.
		resolved, err := celEnv.ResolveResourceMap(ctx, map[string]string{key: expr}, func() (kubeletcel.InstanceTypeVars, error) {
			return vars, nil
		})
		switch {
		case err != nil:
			p.Err = err
		case len(resolved) == 0:
			// ResolveResourceMap drops failures and logs them, so re-evaluate to recover the reason.
			_, evalErr := celEnv.EvaluateExpression(expr, vars)
			if evalErr == nil {
				evalErr = errNegativeResult
			}
			p.Err = evalErr
		default:
			p.Value = resolved[key]
			p.Applied = true
		}
		out = append(out, p)
	}
	return out
}

// errNegativeResult is the drop reason for an expression that evaluated cleanly but produced a negative
// value, which ResolveResourceMap discards without returning an error.
var errNegativeResult = fmt.Errorf("expression evaluated to a negative value, so the entry is dropped")

// maxPodsExpression returns the maxPods CEL expression, if maxPods holds one rather than a static integer.
func maxPodsExpression(kc *v1.KubeletConfiguration) (string, bool) {
	if kc.MaxPods == nil || kc.MaxPods.Type != intstr.String {
		return "", false
	}
	return kc.MaxPods.StrVal, true
}

// sortedExpressionKeys returns the keys of a reserved map whose values are CEL expressions rather than static
// quantities, in a stable order so the tool's output doesn't shuffle between runs. The is-an-expression test
// matches HasResourceExpressions: anything ParseQuantity rejects is treated as an expression.
func sortedExpressionKeys(m map[string]string) []string {
	keys := lo.Filter(lo.Keys(m), func(k string, _ int) bool {
		_, err := resource.ParseQuantity(m[k])
		return err != nil
	})
	sort.Strings(keys)
	return keys
}

// formatCount renders a bare, unitless count (maxPods) for display.
func formatCount(v int64) string {
	return fmt.Sprint(v)
}
