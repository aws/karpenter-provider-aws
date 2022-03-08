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

package binpacking

import (
	"context"
	"fmt"
	"sort"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/apiobject"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/resources"
)

var (
	// MaxInstanceTypes defines the number of instance type options to return to the cloud provider
	MaxInstanceTypes = 20

	packDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metrics.Namespace,
			Subsystem: "allocation_controller",
			Name:      "binpacking_duration_seconds",
			Help:      "Duration of binpacking process in seconds.",
			Buckets:   metrics.DurationBuckets(),
		},
		[]string{metrics.ProvisionerLabel},
	)
)

func init() {
	crmetrics.Registry.MustRegister(packDuration)
}

func NewPacker(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Packer {
	return &Packer{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

// Packer packs pods and calculates efficient placement on the instances.
type Packer struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

// Packing is a binpacking solution of equivalently schedulable pods to a set of
// viable instance types upon which they fit. All pods in the packing are
// within the specified constraints (e.g., labels, taints).
type Packing struct {
	Pods                [][]*v1.Pod `hash:"ignore"`
	NodeQuantity        int         `hash:"ignore"`
	InstanceTypeOptions []cloudprovider.InstanceType
}

// Pack returns the node packings for the provided pods. It computes a set of viable
// instance types for each packing of pods. InstanceType variety enables the cloud provider
// to make better cost and availability decisions. The instance types returned are sorted by resources.
// Pods provided are all schedulable in the same zone as tightly as possible.
// It follows the First Fit Decreasing bin packing technique, reference-
// https://en.wikipedia.org/wiki/First-fit-decreasing_bin_packing
func (p *Packer) Pack(ctx context.Context, constraints *v1alpha5.Constraints, pods []*v1.Pod, instanceTypes []cloudprovider.InstanceType) ([]*Packing, error) {
	defer metrics.Measure(packDuration.WithLabelValues(injection.GetNamespacedName(ctx).Name))()
	// Get daemons for overhead calculations
	daemons, err := p.getDaemons(ctx, constraints)
	if err != nil {
		return nil, fmt.Errorf("getting schedulable daemon pods, %w", err)
	}
	// Sort pods in decreasing order by the amount of CPU requested, if
	// CPU requested is equal compare memory requested.
	sort.Slice(pods, func(a, b int) bool {
		resourcePodA := resources.RequestsForPods(pods[a])
		resourcePodB := resources.RequestsForPods(pods[b])
		if resourcePodA.Cpu().Equal(*resourcePodB.Cpu()) {
			// check for memory
			return resourcePodA.Memory().Cmp(*resourcePodB.Memory()) == 1
		}
		return resourcePodA.Cpu().Cmp(*resourcePodB.Cpu()) == 1
	})
	packs := map[uint64]*Packing{}
	var packings []*Packing
	var packing *Packing
	remainingPods := pods
	emptyPackables := PackablesFor(ctx, instanceTypes, constraints, pods, daemons)
	for len(remainingPods) > 0 {
		packables := []*Packable{}
		for _, packable := range emptyPackables {
			packables = append(packables, packable.DeepCopy())
		}
		if len(packables) == 0 {
			logging.FromContext(ctx).Errorf("Failed to find instance type option(s) for %v", apiobject.PodNamespacedNames(remainingPods))
			return packings, nil
		}
		packing, remainingPods = p.packWithLargestPod(remainingPods, packables)
		// checked all instance types and found no packing option
		if flattenedLen(packing.Pods...) == 0 {
			logging.FromContext(ctx).Errorf("Failed to compute packing, pod(s) %s did not fit in instance type option(s) %v", apiobject.PodNamespacedNames(remainingPods), packableNames(packables))
			remainingPods = remainingPods[1:]
			continue
		}
		key, err := hashstructure.Hash(packing, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
		if err != nil {
			return nil, fmt.Errorf("hashing packings, %w", err)
		}
		if mainPack, ok := packs[key]; ok {
			mainPack.NodeQuantity++
			mainPack.Pods = append(mainPack.Pods, packing.Pods...)
			continue
		}
		packs[key] = packing
		packings = append(packings, packing)
	}
	for _, pack := range packings {
		logging.FromContext(ctx).Infof("Computed packing of %d node(s) for %d pod(s) with instance type option(s) %s", pack.NodeQuantity, flattenedLen(pack.Pods...), instanceTypeNames(pack.InstanceTypeOptions))
	}
	return packings, nil
}

func (p *Packer) getDaemons(ctx context.Context, constraints *v1alpha5.Constraints) ([]*v1.Pod, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	if err := p.kubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}
	// Include DaemonSets that will schedule on this node
	pods := []*v1.Pod{}
	for _, daemonSet := range daemonSetList.Items {
		pod := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
		if err := constraints.ValidatePod(pod); err == nil {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

// packWithLargestPod will try to pack max number of pods with largest pod in
// pods across all available node capacities. It returns Packing: max pod count
// that fit; with their node capacities and list of leftover pods
func (p *Packer) packWithLargestPod(unpackedPods []*v1.Pod, packables []*Packable) (*Packing, []*v1.Pod) {
	bestPackedPods := []*v1.Pod{}
	bestInstances := []cloudprovider.InstanceType{}
	remainingPods := unpackedPods

	// Try to pack the largest instance type to get an upper bound on efficiency
	maxPodsPacked := len(packables[len(packables)-1].DeepCopy().Pack(unpackedPods).packed)
	if maxPodsPacked == 0 {
		return &Packing{Pods: [][]*v1.Pod{bestPackedPods}, InstanceTypeOptions: bestInstances}, remainingPods
	}

	for i, packable := range packables {
		// check how many pods we can fit with the available capacity
		if result := packable.Pack(unpackedPods); len(result.packed) == maxPodsPacked {
			// Add all packable nodes that have more resources than this one
			// Trim the bestInstances so that provisioning APIs in cloud providers are not overwhelmed by the number of instance type options
			// For example, the AWS EC2 Fleet API only allows the request to be 145kb which equates to about 130 instance type options.
			for j := i; j < len(packables) && j-i < MaxInstanceTypes; j++ {
				// packable nodes are sorted lexicographically according to the order of [CPU, memory]
				// It may result in cases where an instance type may have larger index value when it has more CPU but fewer memory
				// Need to exclude instance type with smaller memory and fewer pods
				if packables[i].Memory().Cmp(*packables[j].Memory()) <= 0 && packables[i].Pods().Cmp(*packables[j].Pods()) <= 0 {
					bestInstances = append(bestInstances, packables[j])
				}
			}
			bestPackedPods = result.packed
			remainingPods = result.unpacked
			break
		}
	}
	return &Packing{Pods: [][]*v1.Pod{bestPackedPods}, InstanceTypeOptions: bestInstances, NodeQuantity: 1}, remainingPods
}

func instanceTypeNames(instanceTypes []cloudprovider.InstanceType) []string {
	names := []string{}
	for _, instanceType := range instanceTypes {
		names = append(names, instanceType.Name())
	}
	return names
}

func flattenedLen(pods ...[]*v1.Pod) int {
	length := 0
	for _, ps := range pods {
		length += len(ps)
	}
	return length
}
