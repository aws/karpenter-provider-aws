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
	// 1. Add the Karpenter finalizer to the node to enable the termination workflow
	node.Finalizers = append(node.Finalizers, v1alpha3.TerminationFinalizer)
	// 2. Idempotently create a node. In rare cases, nodes can come online and
	// self register before the controller is able to register a node object
	// with the API server. In the common case, we create the node object
	// ourselves to enforce the binding decision and enable images to be pulled
	// before the node is fully Ready.
	if _, err := b.CoreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating node %s, %w", node.Name, err)
		}
		// If the node object already exists, the finalizer controller will make sure
		// the finalizer is in place, so we can ignore this case here.
	}
	// 3 Partition pods into readiness tolerant (immediately schedulable) and
	// intolerant (not yet schedulable) sets of pods
	intolerantPods := make([]*v1.Pod, 0)
	tolerantPods := make([]*v1.Pod, 0)
	for _, p := range packing.Pods {
		if pod.ToleratesTaints(&p.Spec, packing.Constraints.ReadinessTaints...) == nil {
			tolerantPods = append(tolerantPods, p)
		} else {
			intolerantPods = append(intolerantPods, p)
		}
	}
	// 4. Asynchronously bind intolerant pods if there are any.
	if len(intolerantPods) > 0 {
		go b.asyncBind(ctx, node, packing.Constraints.ReadinessTaints, intolerantPods)
	}
	// 5. Synchronously bind tolerant (immediate) pods
	errs := make([]error, len(tolerantPods))
	workqueue.ParallelizeUntil(ctx, len(tolerantPods), len(tolerantPods), func(index int) {
		errs[index] = b.bind(ctx, node, tolerantPods[index])
	})
	err := multierr.Combine(errs...)
	logging.FromContext(ctx).Infof("Immediately bound %d out of %d pod(s) to node %s", len(tolerantPods)-len(multierr.Errors(err)), len(tolerantPods), node.Name)
	return err
}

func (b *Binder) asyncBind(ctx context.Context, node *v1.Node, readinessTaints []v1.Taint, pods []*v1.Pod) {
	logger := logging.FromContext(ctx)
	logger.Infof("%d pods are not yet schedulable on node %s due to existing node readiness taints", len(pods), node.Name)
	node, err := b.waitForNodeReadiness(ctx, node, readinessTaints)
	if err != nil {
		logger.Warnf("Asynchronous bind failed while waiting for node to become ready, %w", err)
		b.triggerReconcileFor(ctx, pods...)
		return
	}
	errs := make([]error, len(pods))
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(index int) {
		pod := pods[index]
		err := b.bind(ctx, node, pod)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Infof("failed to bind pod %s/%s to node %s, %w", pod.Namespace, pod.Name, node.Name, err)
				b.triggerReconcileFor(ctx, pod)
			} else {
				logger.Debugf("failed to bind pod %s/%s to node %s, %w", pod.Namespace, pod.Name, node.Name, err)
			}
		}
		errs[index] = err
	})
	err = multierr.Combine(errs...)
	logger.Infof("Asynchronously bound %d out of %d pod(s) to node %s", len(pods)-len(multierr.Errors(err)), len(pods), node.Name)
}

func (b *Binder) triggerReconcileFor(ctx context.Context, pods ...*v1.Pod) {
	logger := logging.FromContext(ctx)
	for _, pod := range pods {
		// Touch (update) pod to make sure it triggers another reconciliation for the corresponding
		// provisioner.
		stored := pod.DeepCopy()
		// TODO what exactly shall we update status.Conditions or some metadata.annotation?
		pod.ObjectMeta.Annotations[v1alpha3.AsyncBindFailureAnnotationKey] = time.Now().String()
		err := b.KubeClient.Patch(ctx, pod, client.StrategicMergeFrom(stored), &client.PatchOptions{})
		if err != nil {
			logger.Warnf("failed to update status of pod %s/%s, %w", pod.Namespace, pod.Name, err)
		}
	}
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
		if !hasAnyReadinessTaint(node, readinessTaints) {
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

func hasAnyReadinessTaint(node *v1.Node, readinessTaints []v1.Taint) bool {
	for _, nodeTaint := range node.Spec.Taints {
		// Ignore karpenters own readiness taint
		if nodeTaint.Key == v1alpha3.NotReadyTaintKey {
			continue
		}
		for _, readinessTaint := range readinessTaints {
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
