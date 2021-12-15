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
	"time"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/provisioning/binpacking"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	MaxBatchDuration = time.Second * 10
	MinBatchDuration = time.Second * 1
	// MaxPodsPerBatch limits the number of pods we process at one time to avoid using too much memory
	MaxPodsPerBatch = 2_000
)

func NewProvisioner(ctx context.Context, provisioner *v1alpha5.Provisioner, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Provisioner {
	c, stop := context.WithCancel(ctx)
	p := &Provisioner{
		Provisioner:   provisioner,
		pods:          make(chan *v1.Pod),
		results:       make(chan error),
		done:          c.Done(),
		Stop:          stop,
		cloudProvider: cloudProvider,
		kubeClient:    kubeClient,
		coreV1Client:  coreV1Client,
		scheduler:     scheduling.NewScheduler(kubeClient),
		packer:        binpacking.NewPacker(kubeClient, cloudProvider),
	}
	go func() {
		for {
			select {
			case <-p.done:
				logging.FromContext(ctx).Info("Stopping provisioner")
				return
			default:
				if err := p.provision(ctx); err != nil {
					logging.FromContext(ctx).Errorf("Provisioning failed, %s", err.Error())
				}
			}
		}
	}()
	return p
}

// Provisioner waits for enqueued pods, batches them, creates capacity and binds the pods to the capacity.
type Provisioner struct {
	// State
	*v1alpha5.Provisioner
	pods    chan *v1.Pod
	results chan error
	Stop    context.CancelFunc
	done    <-chan struct{}
	// Dependencies
	cloudProvider cloudprovider.CloudProvider
	kubeClient    client.Client
	coreV1Client  corev1.CoreV1Interface
	scheduler     *scheduling.Scheduler
	packer        *binpacking.Packer
}

func (p *Provisioner) provision(ctx context.Context) (err error) {
	ctx = injection.WithComponentName(ctx, "provisioner")

	// Wait for a batch of pods
	pods := p.Batch(ctx)
	// Communicate the result of the provisioning loop to each of the pods.
	defer func() {
		for i := 0; i < len(pods); i++ {
			select {
			case p.results <- err: // Block until result is communicated
			case <-p.done: // Leave if closed
			}
		}
	}()
	// Separate pods by scheduling constraints
	schedules, err := p.scheduler.Solve(ctx, p.Provisioner, pods)
	if err != nil {
		return fmt.Errorf("solving scheduling constraints, %w", err)
	}
	// Launch capacity and bind pods
	for _, schedule := range schedules {
		packings, err := p.packer.Pack(ctx, schedule.Constraints, schedule.Pods)
		if err != nil {
			return fmt.Errorf("binpacking pods, %w", err)
		}
		for _, packing := range packings {
			if err := p.launch(ctx, schedule.Constraints, packing); err != nil {
				logging.FromContext(ctx).Error("Could not launch node, %s", err.Error())
				continue
			}
		}
	}
	return nil
}

// Add a pod to the provisioner and block until it's processed. The caller
// is responsible for verifying that the pod was scheduled correctly. In the
// future, this may be expanded to include concepts such as retriable errors.
func (p *Provisioner) Add(ctx context.Context, pod *v1.Pod) (err error) {
	select {
	case p.pods <- pod: // Block until pod is enqueued
	case <-p.done: // Leave if closed
	}
	select {
	case err = <-p.results: // Block until result is sent
	case <-p.done: // Leave if closed
	}
	return err
}

// Batch returns a slice of enqueued pods after idle or timeout
func (p *Provisioner) Batch(ctx context.Context) (pods []*v1.Pod) {
	logging.FromContext(ctx).Infof("Waiting for unschedulable pods")
	// Start the batching window after the first pod is received
	pods = append(pods, <-p.pods)
	timeout := time.NewTimer(MaxBatchDuration)
	idle := time.NewTimer(MinBatchDuration)
	start := time.Now()
	defer func() {
		pods = p.FilterProvisionable(ctx, pods)
		logging.FromContext(ctx).Infof("Batched %d pods in %s", len(pods), time.Since(start))
	}()
	for {
		select {
		case pod := <-p.pods:
			idle.Reset(MinBatchDuration)
			pods = append(pods, pod)
			if len(pods) >= MaxPodsPerBatch {
				return pods
			}
		case <-ctx.Done():
			return pods
		case <-timeout.C:
			return pods
		case <-idle.C:
			return pods
		}
	}
}

// FilterProvisionable removes pods that have been assigned a node.
// This check is needed to prevent duplicate binds when a pod is scheduled to a node
// between the time it was ingested into the scheduler and the time it is included
// in a provisioner batch.
func (p *Provisioner) FilterProvisionable(ctx context.Context, pods []*v1.Pod) []*v1.Pod {
	provisionable := []*v1.Pod{}
	for _, pod := range pods {
		// the original pod should be returned rather than the newly fetched pod in case the scheduler relaxed constraints
		original := pod
		candidate := &v1.Pod{}
		if err := p.kubeClient.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}, candidate); err != nil {
			logging.FromContext(ctx).Errorf("Could not verify pod \"%s/%s\" is provisionable, %s", pod.Namespace, pod.Name, err.Error())
			continue
		}
		if candidate.Spec.NodeName == "" {
			provisionable = append(provisionable, original)
		}
	}
	return provisionable
}

func (p *Provisioner) launch(ctx context.Context, constraints *v1alpha5.Constraints, packing *binpacking.Packing) error {
	if err := p.verifyResourceLimits(ctx, p.Provisioner); err != nil {
		return fmt.Errorf("limits exceeded, %w", err)
	}
	packedPods := queueFor(packing.Pods)
	return <-p.cloudProvider.Create(ctx, constraints, packing.InstanceTypeOptions, packing.NodeQuantity, func(node *v1.Node) error {
		node.Labels = functional.UnionStringMaps(node.Labels, constraints.Labels)
		node.Spec.Taints = append(node.Spec.Taints, constraints.Taints...)
		return p.bind(ctx, node, <-packedPods)
	})
}

func (p *Provisioner) bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) (err error) {
	defer metrics.Measure(bindTimeHistogram.WithLabelValues(injection.GetNamespacedName(ctx).Name))()

	// Add the Karpenter finalizer to the node to enable the termination workflow
	node.Finalizers = append(node.Finalizers, v1alpha5.TerminationFinalizer)
	// Taint karpenter.sh/not-ready=NoSchedule to prevent the kube scheduler
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
	if _, err := p.coreV1Client.Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("creating node %s, %w", node.Name, err)
		}
	}
	// Bind pods
	var bound int64
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(i int) {
		pod := pods[i]
		binding := &v1.Binding{TypeMeta: pod.TypeMeta, ObjectMeta: pod.ObjectMeta, Target: v1.ObjectReference{Name: node.Name}}
		if err := p.coreV1Client.Pods(pods[i].Namespace).Bind(ctx, binding, metav1.CreateOptions{}); err != nil {
			logging.FromContext(ctx).Errorf("Failed to bind %s/%s to %s, %s", pod.Namespace, pod.Name, node.Name, err.Error())
		} else {
			atomic.AddInt64(&bound, 1)
		}
	})
	logging.FromContext(ctx).Infof("Bound %d pod(s) to node %s", bound, node.Name)
	return nil
}

func (p *Provisioner) verifyResourceLimits(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	latest := &v1alpha5.Provisioner{}
	if err := p.kubeClient.Get(ctx, client.ObjectKeyFromObject(provisioner), latest); err != nil {
		return fmt.Errorf("getting current resource usage, %w", err)
	}
	return provisioner.Spec.Limits.ExceededBy(latest.Status.Resources)
}

// Thread safe channel to pop off packed pod slices
func queueFor(pods [][]*v1.Pod) <-chan []*v1.Pod {
	queue := make(chan []*v1.Pod, len(pods))
	defer close(queue)
	for _, ps := range pods {
		queue <- ps
	}
	return queue
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
