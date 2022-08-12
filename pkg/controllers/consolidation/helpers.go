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

package consolidation

import (
	"context"
	"fmt"
	"math"
	"strconv"

	cputils "github.com/aws/karpenter/pkg/utils/cloudprovider"

	"github.com/aws/karpenter/pkg/scheduling"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider"
)

// GetPodEvictionCost returns the disruption cost computed for evicting the given pod.
func GetPodEvictionCost(ctx context.Context, p *v1.Pod) float64 {
	cost := 1.0
	podDeletionCostStr, ok := p.Annotations[v1.PodDeletionCost]
	if ok {
		podDeletionCost, err := strconv.ParseFloat(podDeletionCostStr, 64)
		if err != nil {
			logging.FromContext(ctx).Errorf("parsing %s=%s from pod %s, %s",
				v1.PodDeletionCost, podDeletionCostStr, client.ObjectKeyFromObject(p), err)
		} else {
			// the pod deletion disruptionCost is in [-2147483647, 2147483647]
			// the min pod disruptionCost makes one pod ~ -15 pods, and the max pod disruptionCost to ~ 17 pods.
			cost += podDeletionCost / math.Pow(2, 27.0)
		}
	}
	// the scheduling priority is in [-2147483648, 1000000000]
	if p.Spec.Priority != nil {
		cost += float64(*p.Spec.Priority) / math.Pow(2, 25)
	}

	// overall we clamp the pod cost to the range [-10.0, 10.0] with the default being 1.0
	return clamp(-10.0, cost, 10.0)
}

func filterByPrice(options []cloudprovider.InstanceType, reqs scheduling.Requirements, price float64) []cloudprovider.InstanceType {
	var result []cloudprovider.InstanceType
	for _, it := range options {
		cheapestOffering := cputils.CheapestOfferingWithReqs(cputils.AvailableOfferings(it), reqs)
		if cheapestOffering.Price < price {
			result = append(result, it)
		}
	}
	return result
}

func disruptionCost(ctx context.Context, pods []*v1.Pod) float64 {
	cost := 0.0
	for _, p := range pods {
		cost += GetPodEvictionCost(ctx, p)
	}
	return cost
}

// getNodePrice gets the last known node price for the candidate node
// This price is used to determine the savings that can be achieved through consolidation
func getNodePrice(node candidateNode) (float64, error) {
	// Get the last known offering price from the capacity type and zone
	of, err := cputils.GetOffering(node.instanceType.Offerings(), node.capacityType, node.zone)
	if err == nil {
		return of.Price, nil
	}

	// Still need a fallback mechanism if we can't find the offering in our current data
	for _, offering := range node.instanceType.Offerings() {
		if offering.CapacityType == node.capacityType {
			return offering.Price, nil
		}
	}
	return 0, fmt.Errorf("couldn't find an offering price for the passed node")
}
