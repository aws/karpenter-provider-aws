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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Binder struct {
	KubeClient   client.Client
	CoreV1Client corev1.CoreV1Interface
}

func (b *Binder) Bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) error {
	// 1. Add the Karpenter finalizer to the node to enable the termination workflow
	node.Finalizers = append(node.Finalizers, v1alpha3.KarpenterFinalizer)
	// 2. Mark NodeReady=Unknown
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
	// 3. Idempotently create a node. In rare cases, nodes can come online and
	// self register before the controller is able to register a node object
	// with the API server. In the common case, we create the node object
	// ourselves to enforce the binding decision and enable images to be pulled
	// before the node is fully Ready.
	if _, err := b.CoreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating node %s, %w", node.Name, err)
		}
	}

	// 4. Bind pods
	logging.FromContext(ctx).Infof("Binding %d pod(s) to node %s", len(pods), node.Name)
	errs := make([]error, len(pods))
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(index int) {
		errs[index] = b.bind(ctx, node, pods[index])
	})
	return multierr.Combine(errs...)
}

func (b *Binder) bind(ctx context.Context, node *v1.Node, pod *v1.Pod) error {
	// TODO, Stop using deprecated v1.Binding
	if err := b.CoreV1Client.Pods(pod.Namespace).Bind(ctx, &v1.Binding{
		TypeMeta:   pod.TypeMeta,
		ObjectMeta: pod.ObjectMeta,
		Target:     v1.ObjectReference{Name: node.Name},
	}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("binding pod, %w", err)
	}
	return nil
}
