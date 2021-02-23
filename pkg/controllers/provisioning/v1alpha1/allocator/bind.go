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

package allocator

import (
	"context"
	"fmt"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Binder struct {
	kubeClient   client.Client
	coreV1Client corev1.CoreV1Interface
}

func (b *Binder) Bind(ctx context.Context, provisioner *v1alpha1.Provisioner, node *v1.Node, pods []*v1.Pod) error {
	// 1. Decorate node
	b.decorate(provisioner, node)

	// 2. Create node
	if err := b.create(ctx, node); err != nil {
		return err
	}

	// 3. Bind pods
	for _, pod := range pods {
		if err := b.bind(ctx, node, pod); err != nil {
			zap.S().Errorf("Continuing after failing to bind, %s", err.Error())
		} else {
			zap.S().Debugf("Successfully bound pod %s/%s to node %s", pod.Namespace, pod.Name, node.Name)
		}
	}
	return nil
}

func (b *Binder) decorate(provisioner *v1alpha1.Provisioner, node *v1.Node) {
	// 1. Set Labels
	node.Labels = map[string]string{
		v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
		v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
	}
	for key, value := range provisioner.Spec.Allocation.Labels {
		node.Labels[key] = value
	}

	// 2. Set Taints
	node.Spec.Taints = []v1.Taint{}
	node.Spec.Taints = append(node.Spec.Taints, provisioner.Spec.Allocation.Taints...)

	// 3. Set Conditions

	// Unfortunately, this detail is necessary to prevent kube-scheduler from
	// scheduling pods to nodes before they're created. Node Lifecycle
	// Controller will attach a Effect=NoSchedule taint in response to this
	// condition and remove the taint when NodeReady=True. This behavior is
	// stable, but may not be guaranteed to be true in the indefinite future.
	// The failure mode in this case will unnecessarily create additional nodes.
	// https://github.com/kubernetes/kubernetes/blob/f5fb1c93dbaa512eb66090c5027435d3dee95ac7/pkg/controller/nodelifecycle/node_lifecycle_controller.go#L86
	node.Status.Conditions = []v1.NodeCondition{{
		Type:   v1.NodeReady,
		Status: v1.ConditionUnknown,
	}}
}

func (a *Binder) create(ctx context.Context, node *v1.Node) error {
	if _, err := a.coreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("creating node %s, %w", node.Name, err)
	}
	return nil
}

func (a *Binder) bind(ctx context.Context, node *v1.Node, pod *v1.Pod) error {
	// TODO, Stop using deprecated v1.Binding
	if err := a.coreV1Client.Pods(pod.Namespace).Bind(ctx, &v1.Binding{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: pod.ObjectMeta,
		Target:     v1.ObjectReference{Name: node.Name},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("binding pod, %w", err)
	}
	return nil
}
