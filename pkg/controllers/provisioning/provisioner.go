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
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/provisioning/binpacking"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/parallel"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	defaultBatchOptions = parallel.BatchOptions{
		IdleDuration: 1 * time.Second,
		MaxDuration:  10 * time.Second,
		MaxSize:      2_000,
	}
)

func NewProvisioner(ctx context.Context, provisioner *v1alpha5.Provisioner, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Provisioner {
	return &Provisioner{
		Provisioner:   provisioner,
		coreV1Client:  coreV1Client,
		cloudProvider: cloudProvider,
		kubeClient:    kubeClient,
		packer:        binpacking.NewPacker(kubeClient, cloudProvider),
		ctx:           ctx,
	}
}

// Provisioner waits for enqueued pods, batches them, creates capacity and binds the pods to the capacity.
type Provisioner struct {
	// State
	*v1alpha5.Provisioner
	batches sync.Map
	ctx     context.Context
	// Dependencies
	cloudProvider cloudprovider.CloudProvider
	kubeClient    client.Client
	coreV1Client  corev1.CoreV1Interface
	packer        *binpacking.Packer
}

// Add (threadsafe) the pod to the provisioner and block until it's processed.
// The caller is responsible for verifying that the pod was scheduled correctly.
func (p *Provisioner) Add(ctx context.Context, pod *v1.Pod, constraints *v1alpha5.Constraints) {
	// Add to existing batch
	for _, batch := range p.getBatches(constraints) {
		if batch.Add(ctx, pod) {
			return
		}
	}
	// Create a new batch
	batch := parallel.NewBatch(ctx, defaultBatchOptions)
	go func() {
		p.batches.Store(constraints, batch)
		p.provision(p.ctx, constraints, batch)
		p.batches.Delete(constraints)
	}()
	batch.Add(ctx, pod)
}

func (p *Provisioner) getBatches(constraints *v1alpha5.Constraints) []*parallel.Batch {
	result := []*parallel.Batch{}
	p.batches.Range(func(key, value interface{}) bool {
		if constraints.MustHash() == key.(*v1alpha5.Constraints).MustHash() {
			result = append(result, value.(*parallel.Batch))
			return false
		}
		return true
	})
	return result
}

func (p *Provisioner) provision(ctx context.Context, constraints *v1alpha5.Constraints, batch *parallel.Batch) (err error) {
	// Batch pods
	logging.FromContext(ctx).Infof("Waiting for unschedulable pods")
	batch.Start()
	defer batch.Stop()
	pods := []*v1.Pod{}
	for pod := range batch.Next() {
		pods = append(pods, pod.(*v1.Pod))
	}
	logging.FromContext(ctx).Infof("Batched %d pods in %s", len(pods), batch.Duration())
	// Check that the pod wasn't scheduled while waiting for the batch
	pods, err = p.FilterProvisionable(ctx, pods)
	if err != nil {
		return fmt.Errorf("filtering provisionable pods, %w", err)
	}
	// Pack onto instance types
	packings, err := p.packer.Pack(ctx, constraints, pods)
	if err != nil {
		return fmt.Errorf("computing binpackings, %w", err)
	}
	// Launch instances and bind pods
	for _, packing := range packings {
		if err := p.verifyResourceLimits(ctx); err != nil {
			logging.FromContext(ctx).Infof("Packing violated limits, %s", err.Error())
			continue
		}
		packedPods := queueFor(packing.Pods)
		if err := <-p.cloudProvider.Create(ctx, constraints, packing.InstanceTypeOptions, packing.NodeQuantity, func(node *v1.Node) error {
			node.Labels = functional.UnionStringMaps(node.Labels, constraints.Labels)
			node.Spec.Taints = append(node.Spec.Taints, constraints.Taints...)
			return p.bind(ctx, node, <-packedPods)
		}); err != nil {
			return fmt.Errorf("creating capacity, %w", err)
		}
	}
	return nil
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

// FilterProvisionable removes pods that have been assigned a node.
// This check is needed to prevent duplicate binds when a pod is scheduled to a node
// between the time it was ingested into the scheduler and the time it is included
// in a provisioner batch.
func (p *Provisioner) FilterProvisionable(ctx context.Context, pods []*v1.Pod) ([]*v1.Pod, error) {
	provisionable := []*v1.Pod{}
	for _, pod := range pods {
		stored := &v1.Pod{}
		if err := p.kubeClient.Get(ctx, client.ObjectKeyFromObject(pod), stored); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if stored.Spec.NodeName == "" {
			// use the original pod since topology or preferences may have mutated it
			provisionable = append(provisionable, pod)
		}
	}
	return provisionable, nil
}

func (p *Provisioner) verifyResourceLimits(ctx context.Context) error {
	latest := &v1alpha5.Provisioner{}
	if err := p.kubeClient.Get(ctx, client.ObjectKeyFromObject(p.Provisioner), latest); err != nil {
		return fmt.Errorf("getting current resource usage, %w", err)
	}
	return p.Provisioner.Spec.Limits.ExceededBy(latest.Status.Resources)
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
