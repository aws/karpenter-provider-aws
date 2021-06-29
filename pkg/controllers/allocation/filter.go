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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Filter struct {
	kubeClient client.Client
}

func (f *Filter) GetProvisionablePods(ctx context.Context, provisioner *v1alpha2.Provisioner) ([]*v1.Pod, error) {
	// 1. List Pods that aren't scheduled
	pods := &v1.PodList{}
	if err := f.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": ""}); err != nil {
		return nil, fmt.Errorf("listing unscheduled pods, %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, nil
	}

	// 2. Filter pods that aren't provisionable
	provisionable := []*v1.Pod{}
	for _, p := range pods.Items {
		if err := functional.ValidateAll(
			func() error { return f.isUnschedulable(&p) },
			func() error { return f.matchesProvisioner(&p, provisioner) },
			func() error { return f.hasSupportedSchedulingConstraints(&p) },
			func() error { return pod.ToleratesTaints(&p.Spec, provisioner.Spec.Taints...) },
			func() error { return f.withValidConstraints(ctx, &p, provisioner) },
		); err != nil {
			zap.S().Debugf("Ignored pod %s/%s when allocating for provisioner %s/%s, %s",
				p.Name, p.Namespace,
				provisioner.Name, provisioner.Namespace,
				err.Error(),
			)
			continue
		}
		provisionable = append(provisionable, ptr.Pod(p))
	}
	return provisionable, nil
}

func (f *Filter) isUnschedulable(p *v1.Pod) error {
	if !pod.FailedToSchedule(p) {
		return fmt.Errorf("awaiting scheduling")
	}
	if pod.IsOwnedByDaemonSet(p) {
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

func (f *Filter) matchesProvisioner(pod *v1.Pod, provisioner *v1alpha2.Provisioner) error {
	if pod.Spec.NodeSelector == nil {
		return nil
	}
	name, ok := pod.Spec.NodeSelector[v1alpha2.ProvisionerNameLabelKey]
	if !ok {
		return nil
	}
	namespace, ok := pod.Spec.NodeSelector[v1alpha2.ProvisionerNamespaceLabelKey]
	if !ok {
		return nil
	}
	if name == provisioner.Name && namespace == provisioner.Namespace {
		return nil
	}
	return fmt.Errorf("matched another provisioner, %s/%s", name, namespace)
}

func (f *Filter) toleratesTaints(p *v1.Pod, provisioner *v1alpha2.Provisioner) error {
	var err error
	for _, taint := range provisioner.Spec.Taints {
		if err := pod.ToleratesTaints(&p.Spec, taint); err != nil {
			err = multierr.Append(err, fmt.Errorf("did not tolerate %s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return err
}

func (f *Filter) withValidConstraints(ctx context.Context, pod *v1.Pod, provisioner *v1alpha2.Provisioner) error {
	if err := provisioner.Spec.Constraints.WithOverrides(pod).Validate(ctx); err != nil {
		return fmt.Errorf("invalid constraints, %w", err)
	}
	return nil
}
