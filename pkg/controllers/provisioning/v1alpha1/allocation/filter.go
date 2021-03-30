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

package allocation

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
	"github.com/awslabs/karpenter/pkg/utils/scheduling"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Filter struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.Factory
}

func (f *Filter) GetProvisionablePods(ctx context.Context, provisioner *v1alpha1.Provisioner) ([]*v1.Pod, error) {
	// 1. List Pods that aren't scheduled
	pods := &v1.PodList{}
	if err := f.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": ""}); err != nil {
		return nil, fmt.Errorf("listing unscheduled pods, %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, nil
	}

	// 2. Get Supported Labels
	capacity := f.cloudProvider.CapacityFor(&provisioner.Spec)
	architectures, err := capacity.GetArchitectures(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting supported architectures, %w", err)
	}
	operatingSystems, err := capacity.GetOperatingSystems(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting supported operating systems, %w", err)
	}
	zones, err := capacity.GetZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting supported zones, %w", err)
	}
	instanceTypes, err := capacity.GetInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting supported instance types, %w", err)
	}
	capacityTypes, err := capacity.GetCapacityTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting supported capacity types, %w", err)
	}
	supportedLabels := map[string][]string{
		v1alpha1.ArchitectureLabelKey:    architectures,
		v1alpha1.OperatingSystemLabelKey: operatingSystems,
		v1alpha1.ZoneLabelKey:            zones,
		v1alpha1.InstanceTypeLabelKey:    instanceTypes,
		v1alpha1.CapacityTypeLabelKey:    capacityTypes,
	}

	// 2. Filter pods that aren't provisionable
	provisionable := []*v1.Pod{}
	for _, pod := range pods.Items {
		if err := functional.ValidateAll(
			func() error { return f.isUnschedulable(&pod) },
			func() error { return f.matchesProvisioner(&pod, provisioner) },
			func() error { return f.hasSupportedSchedulingConstraints(&pod) },
			func() error { return f.toleratesTaints(&pod, provisioner) },
			func() error { return f.hasSupportedLabels(&pod, supportedLabels) },
		); err != nil {
			zap.S().Debugf("Ignored pod %s/%s when allocating for provisioner %s/%s, %s",
				pod.Name, pod.Namespace,
				provisioner.Name, provisioner.Namespace,
				err.Error(),
			)
			continue
		}
		provisionable = append(provisionable, ptr.Pod(pod))
	}
	return provisionable, nil
}

func (f *Filter) isUnschedulable(pod *v1.Pod) error {
	if !scheduling.FailedToSchedule(pod) {
		return fmt.Errorf("awaiting scheduling")
	}
	if scheduling.IsOwnedByDaemonSet(pod) {
		return fmt.Errorf("owned by daemonset")
	}
	return nil
}

func (f *Filter) hasSupportedSchedulingConstraints(pod *v1.Pod) error {
	if pod.Spec.Affinity != nil {
		return fmt.Errorf("affinity is not supported")
	}
	if pod.Spec.TopologySpreadConstraints != nil {
		return fmt.Errorf("topology spread constraints are not supported")
	}
	return nil
}

func (f *Filter) matchesProvisioner(pod *v1.Pod, provisioner *v1alpha1.Provisioner) error {
	if pod.Spec.NodeSelector == nil {
		return nil
	}
	name, ok := pod.Spec.NodeSelector[v1alpha1.ProvisionerNameLabelKey]
	if !ok {
		return nil
	}
	namespace, ok := pod.Spec.NodeSelector[v1alpha1.ProvisionerNamespaceLabelKey]
	if !ok {
		return nil
	}
	if name == provisioner.Name && namespace == provisioner.Namespace {
		return nil
	}
	return fmt.Errorf("matched another provisioner, %s/%s", name, namespace)
}

func (f *Filter) toleratesTaints(pod *v1.Pod, provisioner *v1alpha1.Provisioner) error {
	var err error
	for _, taint := range provisioner.Spec.Taints {
		if !scheduling.ToleratesTaint(&pod.Spec, taint) {
			err = multierr.Append(err, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return err
}

func (f *Filter) hasSupportedLabels(pod *v1.Pod, supportedLabels map[string][]string) error {
	var err error
	for label, supported := range supportedLabels {
		selected, ok := pod.Spec.NodeSelector[label]
		if !ok {
			continue
		}
		if !functional.ContainsString(supported, selected) {
			err = multierr.Append(err, fmt.Errorf("unsupported value for label %s = %s", label, selected))
		}
	}
	return err
}
