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
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Binder struct {
	KubeClient   client.Client
	CoreV1Client corev1.CoreV1Interface
}

func (b *Binder) Bind(ctx context.Context, node *v1.Node, packing *cloudprovider.Packing) error {
	pods := packing.Pods
	stored := node.DeepCopy()
	// 1. Add the Karpenter finalizer to the node to enable the termination workflow
	node.Finalizers = append(node.Finalizers, v1alpha3.TerminationFinalizer)
	// 2. Taint karpenter.sh/not-ready=NoSchedule to prevent the kube scheduler
	// from scheduling pods before we're able to bind them ourselves. The kube
	// scheduler has an eventually consistent cache of nodes and pods, so it's
	// possible for it to see a provisioned node before it sees the pods bound
	// to it. This creates an edge case where other pending pods may be bound to
	// the node by the kube scheduler, causing OutOfCPU errors when the
	// binpacked pods race to bind to the same node. The system eventually
	// heals, but causes delays from additional provisioning (thrash). This
	// taint will be removed by the node controller when a node is marked ready.
	node.Spec.Taints = append(node.Spec.Taints, v1.Taint{
		Key:    v1alpha3.NotReadyTaintKey,
		Effect: v1.TaintEffectNoSchedule,
	})
	// 3. Idempotently create a node. In rare cases, nodes can come online and
	// self register before the controller is able to register a node object
	// with the API server. In the common case, we create the node object
	// ourselves to enforce the binding decision and enable images to be pulled
	// before the node is fully Ready.
	if _, err := b.CoreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating node %s, %w", node.Name, err)
		}
		// If the node object already exists, make sure finalizer and taint are in place.
		if err := b.KubeClient.Patch(ctx, node, client.StrategicMergeFrom(stored)); err != nil {
			return fmt.Errorf("patching node %s, %w", node.Name, err)
		}
	}
	errs := make([]error, len(pods))
	// 4. Wait for node readiness.
	if len(packing.Constraints.ReadinessTaints) > 0 {
		// 4.1. Partitiony pods into readiness tolerant and intolerant sets of pods
		intolerantPods := make([]*v1.Pod, 0)
		tolerantPods := make([]*v1.Pod, 0)
		for _, p := range pods {
			if pod.ToleratesTaints(&p.Spec, packing.Constraints.ReadinessTaints...) == nil {
				tolerantPods = append(tolerantPods, p)
			} else {
				intolerantPods = append(intolerantPods, p)
			}
		}
		/// 4.2. Bind all Pods which tolerate all readiness taints
		workqueue.ParallelizeUntil(ctx, len(tolerantPods), len(tolerantPods), func(index int) {
			errs[index] = b.bind(ctx, node, tolerantPods[index])
		})
		// 4.3. If there are readiness intolerant pods left, which do not tolerate all readiness taints,
		// wait for all readiness taints to be removed from the node
		pods = intolerantPods
		if len(pods) > 0 {
			var err error
			node, err = b.waitForNodeReadiness(ctx, node, packing.Constraints.ReadinessTaints)
			if err != nil {
				errs = append(errs, err)
				return multierr.Combine(errs...)
			}
		}
	}
	// 5. Bind pods
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(index int) {
		errs[index] = b.bind(ctx, node, pods[index])
	})
	err := multierr.Combine(errs...)
	logging.FromContext(ctx).Infof("Bound %d pod(s) to node %s", len(pods)-len(multierr.Errors(err)), node.Name)
	return err
}

func (b *Binder) waitForNodeReadiness(ctx context.Context, node *v1.Node, readinessTaints []v1.Taint) (*v1.Node, error) {
	//TODO do we need to set timeout here, or does the passed in ctx already have a timeout set?
	//TODO if we set timeout here, what are reasonables values, or shall we make this configurable in the Provisioner spec?
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	nodeWatch, err := b.CoreV1Client.Nodes().Watch(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", node.Name)})
	if err != nil {
		return nil, fmt.Errorf("error watching node %s for readinessTaints removal, %w", node.Name, err)
	}
	defer nodeWatch.Stop()
	for {
		node, err = b.CoreV1Client.Nodes().Get(ctx, node.Name, metav1.GetOptions{})
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("node %s was removed, %w", node.Name, err)
		}
		if !hasAnyTaint(node, readinessTaints) {
			break
		}
		event := <-nodeWatch.ResultChan()
		switch event.Type {
		case watch.Deleted:
			return nil, fmt.Errorf("node %s was removed, %w", node.Name, err)
		case watch.Error:
			err = ctx.Err()
			if err != nil {
				// deadline exceeded or context cancelled, so wait no longer for node readiness
				return nil, fmt.Errorf("error watching node %s for readinessTaints removal, %w", node.Name, err)
			}
			logging.FromContext(ctx).Infof("watcher error while watching node %s for readinessTaints removal, %v", node.Name, event.Object)
			nodeWatch, err = b.CoreV1Client.Nodes().Watch(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", node.Name)})
			if err != nil {
				return nil, fmt.Errorf("error watching node %s for readinessTaints removal, %w", node.Name, err)
			}
		}
	}
	return node, nil
}

func hasAnyTaint(node *v1.Node, taints []v1.Taint) bool {
	for _, nodeTaint := range node.Spec.Taints {
		for _, readinessTaint := range taints {
			if nodeTaint.Key == readinessTaint.Key && nodeTaint.Value == readinessTaint.Value && nodeTaint.Effect == readinessTaint.Effect {
				return true
			}
		}
	}
	return false
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
