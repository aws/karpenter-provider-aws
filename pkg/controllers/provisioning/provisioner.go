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

	"github.com/imdario/mergo"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/pod"
)

func NewProvisioner(ctx context.Context, provisioner *v1alpha5.Provisioner, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Provisioner {
	running, stop := context.WithCancel(ctx)
	p := &Provisioner{
		Provisioner:   provisioner,
		Stop:          stop,
		batcher:       NewBatcher(running),
		cloudProvider: cloudProvider,
		kubeClient:    kubeClient,
		coreV1Client:  coreV1Client,
		scheduler:     scheduling.NewScheduler(kubeClient),
	}
	go func() {
		for running.Err() == nil {
			if err := p.provision(running); err != nil {
				logging.FromContext(running).Errorf("Provisioning failed, %s", err)
			}
		}
		logging.FromContext(running).Info("Stopped provisioner")
	}()
	return p
}

// Provisioner waits for enqueued pods, batches them, creates capacity and binds the pods to the capacity.
type Provisioner struct {
	// State
	*v1alpha5.Provisioner
	batcher *Batcher
	Stop    context.CancelFunc
	// Dependencies
	cloudProvider cloudprovider.CloudProvider
	kubeClient    client.Client
	coreV1Client  corev1.CoreV1Interface
	scheduler     *scheduling.Scheduler
}

// Add a pod to the provisioner and return a channel to block on. The caller is
// responsible for verifying that the pod was scheduled correctly.
func (p *Provisioner) Add(pod *v1.Pod) <-chan struct{} {
	return p.batcher.Add(pod)
}

func (p *Provisioner) provision(ctx context.Context) error {
	// Batch pods
	logging.FromContext(ctx).Infof("Waiting for unschedulable pods")
	items, window := p.batcher.Wait()
	defer p.batcher.Flush()
	// Filter pods
	var pods []*v1.Pod
	for _, item := range items {
		provisionable, err := p.isProvisionable(ctx, item.(*v1.Pod))
		if err != nil {
			return err
		}
		if provisionable {
			pods = append(pods, item.(*v1.Pod))
		}
	}
	if len(pods) == 0 {
		return nil
	}
	logging.FromContext(ctx).Infof("Batched %d pod(s) in %s", len(pods), window)

	// Get instance type options
	instanceTypes, err := p.cloudProvider.GetInstanceTypes(ctx, p.Spec.Provider)
	if err != nil {
		return fmt.Errorf("getting instance types, %w", err)
	}

	// Separate pods by scheduling constraints
	nodes, err := p.scheduler.Solve(ctx, &p.Provisioner.Spec.Constraints, instanceTypes, pods)
	if err != nil {
		return fmt.Errorf("solving scheduling constraints, %w", err)
	}

	// Launch capacity and bind pods
	workqueue.ParallelizeUntil(ctx, len(nodes), len(nodes), func(i int) {
		if err := p.launch(ctx, nodes[i]); err != nil {
			logging.FromContext(ctx).Errorf("Launching node, %s", err)
		}
	})
	return nil
}

// isProvisionable ensure that the pod can still be provisioned.
// This check is needed to prevent duplicate binds when a pod is scheduled to a node
// between the time it was ingested into the scheduler and the time it is included
// in a provisioner batch.
func (p *Provisioner) isProvisionable(ctx context.Context, candidate *v1.Pod) (bool, error) {
	stored := &v1.Pod{}
	if err := p.kubeClient.Get(ctx, client.ObjectKeyFromObject(candidate), stored); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return !pod.IsScheduled(stored), nil
}

func (p *Provisioner) launch(ctx context.Context, node *scheduling.Node) error {
	// Check limits
	latest := &v1alpha5.Provisioner{}
	if err := p.kubeClient.Get(ctx, client.ObjectKeyFromObject(p.Provisioner), latest); err != nil {
		return fmt.Errorf("getting current resource usage, %w", err)
	}
	if err := p.Spec.Limits.ExceededBy(latest.Status.Resources); err != nil {
		return err
	}
	k8sNode, err := p.cloudProvider.Create(ctx, &cloudprovider.NodeRequest{
		InstanceTypeOptions: node.InstanceTypeOptions,
		Template: &cloudprovider.NodeTemplate{
			Provider:             p.Spec.Provider,
			Labels:               node.Constraints.Labels,
			Taints:               node.Constraints.Taints,
			Requirements:         node.Constraints.Requirements,
			KubeletConfiguration: node.Constraints.KubeletConfiguration,
		},
	})
	if err != nil {
		return fmt.Errorf("creating cloud provider machine, %w", err)
	}

	if err := mergo.Merge(k8sNode, node.Constraints.ToNode()); err != nil {
		return fmt.Errorf("merging cloud provider node, %w", err)
	}
	// Idempotently create a node. In rare cases, nodes can come online and
	// self register before the controller is able to register a node object
	// with the API server. In the common case, we create the node object
	// ourselves to enforce the binding decision and enable images to be pulled
	// before the node is fully Ready.
	if _, err := p.coreV1Client.Nodes().Create(ctx, k8sNode, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logging.FromContext(ctx).Debugf("node %s already registered", k8sNode.Name)
		} else {
			return fmt.Errorf("creating node %s, %w", k8sNode.Name, err)
		}
	}
	logging.FromContext(ctx).Infof("Created %s", node)
	if err := p.bind(ctx, k8sNode, node.Pods); err != nil {
		return fmt.Errorf("binding pods, %w", err)
	}
	return nil
}

func (p *Provisioner) bind(ctx context.Context, node *v1.Node, pods []*v1.Pod) (err error) {
	defer metrics.Measure(bindTimeHistogram.WithLabelValues(injection.GetNamespacedName(ctx).Name))()
	// Bind pods
	workqueue.ParallelizeUntil(ctx, len(pods), len(pods), func(i int) {
		if err := p.coreV1Client.Pods(pods[i].Namespace).Bind(ctx, &v1.Binding{TypeMeta: pods[i].TypeMeta, ObjectMeta: pods[i].ObjectMeta, Target: v1.ObjectReference{Name: node.Name}}, metav1.CreateOptions{}); err != nil {
			logging.FromContext(ctx).Errorf("Failed to bind %s/%s to %s, %s", pods[i].Namespace, pods[i].Name, node.Name, err)
		}
	})
	return nil
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
