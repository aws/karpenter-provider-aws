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

package termination

import (
	"context"
	"fmt"

	provisioning "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"github.com/awslabs/karpenter/pkg/utils/ptr"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Terminator struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	coreV1Client  corev1.CoreV1Interface
}

// cordon cordons a node
func (t *Terminator) cordon(ctx context.Context, node *v1.Node) error {
	// 1. Check if node is already cordoned
	if node.Spec.Unschedulable {
		return nil
	}
	// 2. Cordon node
	persisted := node.DeepCopy()
	node.Spec.Unschedulable = true
	if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
		return fmt.Errorf("patching node %s, %w", node.Name, err)
	}
	zap.S().Debugf("Cordoned node %s", node.Name)
	return nil
}

// drain evicts pods from the node and returns true when all pods are evicted
func (t *Terminator) drain(ctx context.Context, node *v1.Node) (bool, error) {
	// 1. Get pods on node
	pods, err := t.getPods(ctx, node)
	if err != nil {
		return false, fmt.Errorf("listing pods for node %s, %w", node.Name, err)
	}
	// 2. Separate pods as non-critical and critical
	// https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown
	nonCritical := []*v1.Pod{}
	critical := []*v1.Pod{}
	for _, p := range pods {
		if val := p.Annotations[provisioning.KarpenterDoNotEvictPodAnnotation]; val == "true" {
			zap.S().Debugf("Unable to drain node %s, pod %s has do-not-evict annotation", node.Name, p.Name)
			return false, nil
		}
		// If a pod tolerates the unschedulable taint, don't evict it as it could reschedule back onto the node
		if err := pod.ToleratesTaints(&p.Spec, v1.Taint{Key: v1.TaintNodeUnschedulable, Effect: v1.TaintEffectNoSchedule}); err == nil {
			continue
		}
		if p.Spec.PriorityClassName == "system-cluster-critical" || p.Spec.PriorityClassName == "system-node-critical" {
			critical = append(critical, p)
		} else {
			nonCritical = append(nonCritical, p)
		}
	}
	// 3. Evict non-critical pods
	if !t.evictPods(ctx, nonCritical) {
		return false, nil
	}
	// 4. Evict critical pods once all non-critical pods are evicted
	if !t.evictPods(ctx, critical) {
		return false, nil
	}
	return true, nil
}

// terminate terminates the node then removes the finalizer to delete the node
func (t *Terminator) terminate(ctx context.Context, node *v1.Node) error {
	// 1. Terminate instance associated with node
	if err := t.cloudProvider.Terminate(ctx, node); err != nil {
		return fmt.Errorf("terminating cloudprovider instance, %w", err)
	}
	zap.S().Infof("Terminated instance %s", node.Name)
	// 2. Remove finalizer from node in APIServer
	persisted := node.DeepCopy()
	node.Finalizers = functional.StringSliceWithout(node.Finalizers, provisioning.KarpenterFinalizer)
	if err := t.kubeClient.Patch(ctx, node, client.MergeFrom(persisted)); err != nil {
		return fmt.Errorf("removing finalizer from node %s, %w", node.Name, err)
	}
	return nil
}

// evictPods returns true if there are no evictable pods
func (t *Terminator) evictPods(ctx context.Context, pods []*v1.Pod) bool {
	for _, p := range pods {
		if err := t.coreV1Client.Pods(p.Namespace).Evict(ctx, &v1beta1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: p.Namespace}}); err != nil {
			// If an eviction fails, we need to eventually try again
			zap.S().Debugf("Continuing after failing to evict pod %s from node %s, %s", p.Name, p.Spec.NodeName, err.Error())
		}
	}
	return len(pods) == 0
}

// getPods returns a list of pods scheduled to a node based on some filters
func (t *Terminator) getPods(ctx context.Context, node *v1.Node) ([]*v1.Pod, error) {
	pods := &v1.PodList{}
	if err := t.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return nil, fmt.Errorf("listing pods on node %s, %w", node.Name, err)
	}
	return ptr.PodListToSlice(pods), nil
}
