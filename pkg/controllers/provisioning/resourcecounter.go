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

package provisioning

import (
	"context"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"gopkg.in/inf.v0"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceCount struct {
	CPU    *inf.Dec
	Memory *inf.Dec
}

type Limiter struct {
	KubeClient client.Client
}

// Constructs a new instance of the resource counter
func NewResourceCounter(kubeClient client.Client) *ResourceCounter {
	return &ResourceCounter{
		KubeClient: kubeClient,
	}
}

func (r *ResourceCounter) remainingResourceCounts(ctx context.Context, limits v1alpha5.Limits, provisionerName string) (*ResourceCount, error) {
	// We recalculate remaining resources each provisioning loop, because the terminate controller could've freed up some capacity since
	// we last provisioned worker nodes.
	nodeList := v1.NodeList{}
	withProvisionerName := client.MatchingLabels{v1alpha5.ProvisionerNameLabelKey: provisionerName}
	if err := r.KubeClient.List(ctx, &nodeList, withProvisionerName); err != nil {
		return nil, err
	}
	resourceCount := ResourceCount{
		CPU:    resource.Zero.AsDec(),
		Memory: resource.Zero.AsDec(),
	}
	for _, node := range nodeList.Items {
		resourceCount.CPU.Add(resourceCount.CPU, node.Status.Capacity.Cpu().AsDec())
		resourceCount.Memory.Add(resourceCount.Memory, node.Status.Capacity.Memory().AsDec())
	}

	limits = *limits.DeepCopy()
	remainingResources := &ResourceCount{
		CPU:    limits.Resources.CPU.AsDec().Sub(limits.Resources.CPU.AsDec(), resourceCount.CPU),
		Memory: limits.Resources.Memory.AsDec().Sub(limits.Resources.Memory.AsDec(), resourceCount.Memory),
	}
	return remainingResources, nil
}

// Reduces the resource counts based on how many resources a node uses.
func (r *ResourceCount) updateCountsFor(node *v1.Node) *ResourceCount {
	r.CPU.Sub(r.CPU, node.Status.Capacity.Cpu().AsDec())
	r.Memory.Sub(r.Memory, node.Status.Capacity.Memory().AsDec())
	return r
}

func (r *ResourceCount) isInsufficient(ctx context.Context) bool {
	if r.CPU.Cmp(resource.Zero.AsDec()) <= 0 {
		logging.FromContext(ctx).Errorf("cpu limits breached")
		return true
	} else if r.Memory.Cmp(resource.Zero.AsDec()) <= 0 {
		logging.FromContext(ctx).Errorf("memory limits breached")
		return true
	}
	return false
}
