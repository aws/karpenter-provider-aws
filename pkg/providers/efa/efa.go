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

// Package efa decides whether a launch attaches EFA interfaces. The instance provider and the launch template resolver
// share one Provider so the EFA count recorded on the instance always agrees with the interfaces the launch template
// attaches.
package efa

import (
	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/efadra"
)

// Provider decides whether nodeClaim's instance is launched with EFA interfaces. When it is, every EFA interface the
// instance type supports is attached, so the decision is a boolean rather than a count.
type Provider interface {
	Enabled(nodeClaim *karpv1.NodeClaim, nodeClass *v1.EC2NodeClass) bool
}

// DefaultProvider enables EFA when the NodeClaim requests the EFA extended resource or was allocated dra.net devices.
type DefaultProvider struct{}

func (DefaultProvider) Enabled(nodeClaim *karpv1.NodeClaim, _ *v1.EC2NodeClass) bool {
	return lo.Contains(lo.Keys(nodeClaim.Spec.Resources.Requests), v1.ResourceEFA) || efadra.Requested(nodeClaim)
}
