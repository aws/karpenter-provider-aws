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

package v1alpha4

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/scheduling"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
)

// SetDefaults for the provisioner
func (p *Provisioner) SetDefaults(ctx context.Context) {
	p.Spec.Constraints.Default(ctx)
}

// Default the constraints
func (c *Constraints) Default(ctx context.Context) {
	DefaultHook(ctx, c)
}

// Constrain applies the pods' scheduling constraints to the constraints.
// Returns an error if the constraints cannot be applied.
func (c *Constraints) Constrain(ctx context.Context, pods ...*v1.Pod) (errs error) {
	nodeAffinity := scheduling.NodeAffinityFor(pods...)
	for label, constraint := range map[string]*[]string{
		v1.LabelTopologyZone:       &c.Zones,
		v1.LabelInstanceTypeStable: &c.InstanceTypes,
		v1.LabelArchStable:         &c.Architectures,
		v1.LabelOSStable:           &c.OperatingSystems,
	} {
		values := nodeAffinity.GetLabelValues(label, *constraint, WellKnownLabels[label])
		if len(values) == 0 {
			errs = multierr.Append(errs, fmt.Errorf("label %s is too constrained", label))
		}
		*constraint = values
	}
	return multierr.Append(errs, ConstrainHook(ctx, c, pods...))
}
