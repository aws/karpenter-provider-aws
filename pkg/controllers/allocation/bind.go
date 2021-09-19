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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/metrics"
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

var bindTimeHistogramVec = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.KarpenterNamespace,
		Subsystem: "allocation_controller",
		Name:      "bind_duration_seconds",
		Help:      "Duration of bind process in seconds. Broken down by result.",
		Buckets:   metrics.DurationBuckets(),
	},
	[]string{metrics.ResultLabel},
)

func init() {
	crmetrics.Registry.MustRegister(bindTimeHistogramVec)
}

type Binder struct {
	KubeClient   client.Client
	CoreV1Client corev1.CoreV1Interface
}

func (b *Binder) Bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) error {
	startTime := time.Now()
	bindErr := b.bind(ctx, node, pods)
	durationSeconds := time.Since(startTime).Seconds()

	result := "success"
	if bindErr != nil {
		result = "error"
	}

	labels := prometheus.Labels{metrics.ResultLabel: result}
	observer, promErr := bindTimeHistogramVec.GetMetricWith(labels)
	if promErr != nil {
		logging.FromContext(ctx).Warnf(
			"Failed to record bind duration metric [labels=%s, duration=%f]: error=%s",
			labels,
			durationSeconds,
			promErr.Error(),
		)
	} else {
		observer.Observe(durationSeconds)
	}

	return bindErr
}

func (b *Binder) bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) error {
	// 1. Add the Karpenter finalizer to the node to enable the termination workflow
	node.Finalizers = append(node.Finalizers, v1alpha4.TerminationFinalizer)
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
		Key:    v1alpha4.NotReadyTaintKey,
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
	}

	// 4. Bind pods
	errs := make([]error, len(pods))
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(index int) {
		errs[index] = b.bindPod(ctx, node, pods[index])
	})
	err := multierr.Combine(errs...)
	logging.FromContext(ctx).Infof("Bound %d pod(s) to node %s", len(pods)-len(multierr.Errors(err)), node.Name)
	return err
}

func (b *Binder) bindPod(ctx context.Context, node *v1.Node, pod *v1.Pod) error {
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
