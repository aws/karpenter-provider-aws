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

package packing

import (
	"context"
	"math"
	"sort"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/apiobject"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// MaxInstanceTypes defines the number of instance type options to return to the cloud provider
	MaxInstanceTypes = 20
)

// Constraints for an efficient binpacking solution of pods onto nodes, given
// overhead and node constraints.
type Constraints struct {
	*v1alpha3.Constraints
	// Pods is a list of equivalently schedulable pods to be binpacked.
	Pods []*v1.Pod
	// Daemons resources per node.
	Daemons []*v1.Pod
}

type packer struct{}

// Packer helps pack the pods and calculates efficient placement on the instances.
type Packer interface {
	Pack(context.Context, *Constraints, []cloudprovider.InstanceType) []*cloudprovider.Packing
}

// NewPacker returns a Packer implementation
func NewPacker() Packer {
	return &packer{}
}

// Pack returns the node packings for the provided pods. It computes a set of viable
// instance types for each packing of pods. InstanceType variety enables the cloud provider
// to make better cost and availability decisions. The instance types returned are sorted by resources.
// Pods provided are all schedulable in the same zone as tightly as possible.
// It follows the First Fit Decreasing bin packing technique, reference-
// https://en.wikipedia.org/wiki/Bin_packing_problem#First_Fit_Decreasing_(FFD)
func (p *packer) Pack(ctx context.Context, constraints *Constraints, instances []cloudprovider.InstanceType) []*cloudprovider.Packing {
	// Sort pods in decreasing order by the amount of CPU requested, if
	// CPU requested is equal compare memory requested.
	sort.Sort(sort.Reverse(ByResourcesRequested{SortablePods: constraints.Pods}))
	var packings []*cloudprovider.Packing
	var packing *cloudprovider.Packing
	remainingPods := constraints.Pods
	for len(remainingPods) > 0 {

		packing, remainingPods = p.packWithLargestPod(ctx, remainingPods, constraints, instances)
		// checked all instance types and found no packing option
		if len(packing.Pods) == 0 {
			logging.FromContext(ctx).Errorf("Failed to compute packing for pod(s) %v with instance type option(s) %v", apiobject.PodNamespacedNames(remainingPods), instanceTypeNames(instances))
			remainingPods = remainingPods[1:]
			continue
		}
		packings = append(packings, packing)
		logging.FromContext(ctx).Infof("Computed packing for %d pod(s) with instance type option(s) %s", len(packing.Pods), instanceTypeNames(packing.InstanceTypeOptions))
	}
	return packings
}

// packWithLargestPod will try to pack max number of pods with largest pod in
// pods across all available node capacities. It returns Packing: max pod count
// that fit; with their node capacities and list of leftover pods
func (p *packer) packWithLargestPod(ctx context.Context, unpackedPods []*v1.Pod, constraints *Constraints, instances []cloudprovider.InstanceType) (*cloudprovider.Packing, []*v1.Pod) {
	bestPackedPods := []*v1.Pod{}
	bestInstances := []cloudprovider.InstanceType{}
	remainingPods := unpackedPods
	for _, packable := range PackablesFor(ctx, instances, constraints) {
		// check how many pods we can fit with the available capacity
		result := packable.Pack(unpackedPods)
		if len(result.packed) == 0 {
			continue
		}
		// If the pods packed are the same as before, this instance type can be
		// considered as a backup option in case we get ICE
		if p.podsMatch(bestPackedPods, result.packed) {
			bestInstances = append(bestInstances, packable.InstanceType)
		} else if len(result.packed) > len(bestPackedPods) {
			// If pods packed are more than compared to what we got in last
			// iteration, consider using this instance type
			bestPackedPods = result.packed
			remainingPods = result.unpacked
			bestInstances = []cloudprovider.InstanceType{packable.InstanceType}
		}
	}
	sortByResources(bestInstances)
	// Trim the bestInstances so that provisioning APIs in cloud providers are not overwhelmed by the number of instance type options
	// For example, the AWS EC2 Fleet API only allows the request to be 145kb which equates to about 130 instance type options.
	if len(bestInstances) > MaxInstanceTypes {
		bestInstances = bestInstances[:MaxInstanceTypes]
	}
	return &cloudprovider.Packing{Pods: bestPackedPods, Constraints: constraints.Constraints, InstanceTypeOptions: bestInstances}, remainingPods
}

func (*packer) podsMatch(first, second []*v1.Pod) bool {
	if len(first) != len(second) {
		return false
	}
	podSeen := map[string]int{}
	for _, pod := range first {
		podSeen[client.ObjectKeyFromObject(pod).String()]++
	}
	for _, pod := range second {
		podSeen[client.ObjectKeyFromObject(pod).String()]--
	}
	for _, value := range podSeen {
		if value != 0 {
			return false
		}
	}
	return true
}

// sortByResources sorts instance types, selecting smallest first. Instance are
// ordered using a weighted euclidean, a useful algorithm for reducing a high
// dimesional space into a single heuristic value. In the future, we may explore
// pricing APIs to explicitly order what the euclidean is estimating.
func sortByResources(instanceTypes []cloudprovider.InstanceType) {
	sort.Slice(instanceTypes, func(i, j int) bool { return weightOf(instanceTypes[i]) < weightOf(instanceTypes[j]) })
}

// weightOf uses a euclidean distance function to compare the instance types.
// Units are normalized such that 1cpu = 1gb mem. Additionally, accelerators
// carry an arbitrarily large weight such that they will dominate the priority,
// but if equal, will still fall back to the weight of other dimensions.
func weightOf(instanceType cloudprovider.InstanceType) float64 {
	return euclidean(
		float64(instanceType.CPU().Value()),
		float64(instanceType.Memory().ScaledValue(resource.Mega)), // 1 gb = 1 cpu
		float64(instanceType.NvidiaGPUs().Value())*1000,           // Heavily weigh gpus x 1000
		float64(instanceType.AWSNeurons().Value())*1000,           // Heavily weigh neurons x1000
	)
}

// euclidean measures the n-dimensional distance from the origin.
func euclidean(values ...float64) float64 {
	sum := float64(0)
	for _, value := range values {
		sum += math.Pow(value, 2)
	}
	return math.Pow(sum, .5)
}

func instanceTypeNames(instanceTypes []cloudprovider.InstanceType) []string {
	names := []string{}
	for _, instanceType := range instanceTypes {
		names = append(names, instanceType.Name())
	}
	return names
}

type SortablePods []*v1.Pod

func (pods SortablePods) Len() int {
	return len(pods)
}

func (pods SortablePods) Swap(i, j int) {
	pods[i], pods[j] = pods[j], pods[i]
}

type ByResourcesRequested struct{ SortablePods }

func (r ByResourcesRequested) Less(a, b int) bool {
	resourcePodA := resources.RequestsForPods(r.SortablePods[a])
	resourcePodB := resources.RequestsForPods(r.SortablePods[b])
	if resourcePodA.Cpu().Equal(*resourcePodB.Cpu()) {
		// check for memory
		return resourcePodA.Memory().Cmp(*resourcePodB.Memory()) == -1
	}
	return resourcePodA.Cpu().Cmp(*resourcePodB.Cpu()) == -1
}
