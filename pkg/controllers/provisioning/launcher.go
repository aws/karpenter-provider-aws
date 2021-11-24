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

package provisioning

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/provisioning/binpacking"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type Launcher struct {
	Packer        *binpacking.Packer
	KubeClient    client.Client
	CoreV1Client  corev1.CoreV1Interface
	CloudProvider cloudprovider.CloudProvider
}

func (l *Launcher) Launch(ctx context.Context, provisioner *v1alpha5.Provisioner, schedules []*scheduling.Schedule,
	instanceTypes []cloudprovider.InstanceType) error {
	// Pack and bind pods
	errs := make([]error, len(schedules))
	workqueue.ParallelizeUntil(ctx, len(schedules), len(schedules), func(index int) {
		for _, packing := range l.Packer.Pack(ctx, schedules[index], instanceTypes) {
			if err := l.verifyResourceLimits(ctx, provisioner); err != nil {
				errs[index] = multierr.Append(errs[index], fmt.Errorf("verifying limits, %w", err))
				continue
			}
			// Create thread safe channel to pop off packed pod slices
			packedPods := make(chan []*v1.Pod, len(packing.Pods))
			for _, pods := range packing.Pods {
				packedPods <- pods
			}
			close(packedPods)
			if err := <-l.CloudProvider.Create(ctx, packing.Constraints, packing.InstanceTypeOptions, packing.NodeQuantity, func(node *v1.Node) error {
				node.Labels = functional.UnionStringMaps(node.Labels, packing.Constraints.Labels)
				node.Spec.Taints = append(node.Spec.Taints, packing.Constraints.Taints...)
				return l.bind(ctx, node, <-packedPods)
			}); err != nil {
				errs[index] = multierr.Append(errs[index], err)
			}
		}
	})
	return multierr.Combine(errs...)
}

func (l *Launcher) bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) (err error) {
	defer metrics.Measure(bindTimeHistogram.WithLabelValues(injection.GetNamespacedName(ctx).Name))()

	// Add the Karpenter finalizer to the node to enable the termination workflow
	node.Finalizers = append(node.Finalizers, v1alpha5.TerminationFinalizer)
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
		Key:    v1alpha5.NotReadyTaintKey,
		Effect: v1.TaintEffectNoSchedule,
	})
	// Idempotently create a node. In rare cases, nodes can come online and
	// self register before the controller is able to register a node object
	// with the API server. In the common case, we create the node object
	// ourselves to enforce the binding decision and enable images to be pulled
	// before the node is fully Ready.
	if _, err := l.CoreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating node %s, %w", node.Name, err)
		}
	}
	// Bind pods
	var bound int64
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(i int) {
		pod := pods[i]
		binding := &v1.Binding{TypeMeta: pod.TypeMeta, ObjectMeta: pod.ObjectMeta, Target: v1.ObjectReference{Name: node.Name}}
		if err := l.CoreV1Client.Pods(pods[i].Namespace).Bind(ctx, binding, metav1.CreateOptions{}); err != nil {
			logging.FromContext(ctx).Errorf("Failed to bind %s/%s to %s, %s", pod.Namespace, pod.Name, node.Name, err.Error())
		} else {
			atomic.AddInt64(&bound, 1)
		}
	})
	logging.FromContext(ctx).Infof("Bound %d pod(s) to node %s", bound, node.Name)
	return nil
}

func (l *Launcher) verifyResourceLimits(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	latest := &v1alpha5.Provisioner{}
	if err := l.KubeClient.Get(ctx, client.ObjectKeyFromObject(provisioner), latest); err != nil {
		return fmt.Errorf("getting current resource usage, %w", err)
	}
	return provisioner.Spec.Limits.ExceededBy(latest.Status.Resources)
}

var bindTimeHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "allocation_controller",
		Name:      "bind_duration_seconds",
		Help:      "Duration of bind process in seconds. Broken down by result.",
		Buckets:   metrics.DurationBuckets(),
	},
	[]string{metrics.ProvisionerLabel},
)

func init() {
	crmetrics.Registry.MustRegister(bindTimeHistogram)
}
