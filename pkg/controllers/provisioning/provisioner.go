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

	"github.com/imdario/mergo"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/multierr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/config"
	scheduler "github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/utils/resources"
)

func NewProvisioner(ctx context.Context, cfg config.Config, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, recorder events.Recorder, cloudProvider cloudprovider.CloudProvider, cluster *state.Cluster) *Provisioner {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("provisioning"))
	running, stop := context.WithCancel(ctx)
	p := &Provisioner{
		Stop:           stop,
		batcher:        NewBatcher(running, cfg),
		cloudProvider:  cloudProvider,
		kubeClient:     kubeClient,
		coreV1Client:   coreV1Client,
		volumeTopology: NewVolumeTopology(kubeClient),
		cluster:        cluster,
		recorder:       recorder,
	}
	p.cond = sync.NewCond(&p.mu)
	go p.Start(running)
	return p
}

// Provisioner waits for enqueued pods, batches them, creates capacity and binds the pods to the capacity.
type Provisioner struct {
	// State
	Stop context.CancelFunc
	// Dependencies
	cloudProvider  cloudprovider.CloudProvider
	kubeClient     client.Client
	coreV1Client   corev1.CoreV1Interface
	batcher        *Batcher
	volumeTopology *VolumeTopology
	cluster        *state.Cluster
	recorder       events.Recorder

	mu   sync.Mutex
	cond *sync.Cond
}

func (p *Provisioner) Trigger() {
	p.batcher.Trigger()
}

// Deprecated: TriggerAndWait is used for unit testing purposes only
func (p *Provisioner) TriggerAndWait() {
	p.mu.Lock()
	p.batcher.TriggerImmediate()
	p.cond.Wait()
	p.mu.Unlock()
}

func (p *Provisioner) Start(ctx context.Context) {
	for ctx.Err() == nil {
		if errs := p.Provision(ctx); errs != nil {
			for _, err := range multierr.Errors(errs) {
				logging.FromContext(ctx).Errorf("Provisioning failed, %s", err)
			}
		}
	}
	logging.FromContext(ctx).Info("Stopped provisioner")
}

func (p *Provisioner) Provision(ctx context.Context) error {
	// Batch pods
	p.batcher.Wait()
	// wake any waiters on the cond
	defer p.cond.Broadcast()

	// Get pods, exit if nothing to do
	pods, err := p.getPods(ctx)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return nil
	}
	// Schedule pods to potential nodes, exit if nothing to do
	nodes, err := p.schedule(ctx, pods)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	// Launch capacity and bind pods
	errs := make([]error, len(nodes))
	workqueue.ParallelizeUntil(ctx, len(nodes), len(nodes), func(i int) {
		// create a new context to avoid a data race on the ctx variable
		ctx := logging.WithLogger(ctx, logging.FromContext(ctx).With("provisioner", nodes[i].Labels[v1alpha5.ProvisionerNameLabelKey]))
		// register the provisioner on the context so we can pull it off for tagging purposes
		// TODO: rethink this, maybe just pass the provisioner down instead of hiding it in the context?
		ctx = injection.WithNamespacedName(ctx, types.NamespacedName{Name: nodes[i].Labels[v1alpha5.ProvisionerNameLabelKey]})
		if err := p.launch(ctx, nodes[i]); err != nil {
			errs[i] = fmt.Errorf("launching node, %w", err)
		}
	})
	if err := multierr.Combine(errs...); err != nil {
		return err
	}
	logging.FromContext(ctx).Infof("Waiting for unschedulable pods")
	return nil
}

func (p *Provisioner) getPods(ctx context.Context) ([]*v1.Pod, error) {
	var podList v1.PodList
	if err := p.kubeClient.List(ctx, &podList, client.MatchingFields{"spec.nodeName": ""}); err != nil {
		return nil, fmt.Errorf("listing pods, %w", err)
	}
	var pods []*v1.Pod
	for i := range podList.Items {
		po := podList.Items[i]
		// filter for provisionable pods first so we don't check for validity/PVCs on pods we won't provision anyway
		// (e.g. those owned by daemonsets)
		if !pod.IsProvisionable(&po) {
			continue
		}
		if err := p.Validate(ctx, &po); err != nil {
			logging.FromContext(ctx).With("pod", client.ObjectKeyFromObject(&po)).Debugf("Ignoring pod, %s", err)
			continue
		}
		pods = append(pods, &po)
	}
	return pods, nil
}

// nolint: gocyclo
func (p *Provisioner) schedule(ctx context.Context, pods []*v1.Pod) ([]*scheduler.Node, error) {
	defer metrics.Measure(schedulingDuration.WithLabelValues(injection.GetNamespacedName(ctx).Name))()

	// Build node templates
	var nodeTemplates []*scheduling.NodeTemplate
	var provisionerList v1alpha5.ProvisionerList
	instanceTypes := map[string][]cloudprovider.InstanceType{}
	domains := map[string]sets.String{}
	if err := p.kubeClient.List(ctx, &provisionerList); err != nil {
		return nil, fmt.Errorf("listing provisioners, %w", err)
	}
	for i := range provisionerList.Items {
		provisioner := &provisionerList.Items[i]
		if !provisioner.DeletionTimestamp.IsZero() {
			continue
		}
		// Create node template
		nodeTemplates = append(nodeTemplates, scheduling.NewNodeTemplate(provisioner))
		// Get instance type options
		instanceTypeOptions, err := p.cloudProvider.GetInstanceTypes(ctx, provisioner)
		if err != nil {
			return nil, fmt.Errorf("getting instance types, %w", err)
		}
		instanceTypes[provisioner.Name] = append(instanceTypes[provisioner.Name], instanceTypeOptions...)
		// Construct Topology Domains
		for _, instanceType := range instanceTypeOptions {
			for key, requirement := range instanceType.Requirements() {
				domains[key] = domains[key].Union(requirement.Values())
			}
		}
	}
	if len(nodeTemplates) == 0 {
		return nil, fmt.Errorf("no provisioners found")
	}

	// Inject topology requirements
	for _, pod := range pods {
		if err := p.volumeTopology.Inject(ctx, pod); err != nil {
			return nil, fmt.Errorf("getting volume topology requirements, %w", err)
		}
	}

	// Calculate cluster topology
	topology, err := scheduler.NewTopology(ctx, p.kubeClient, p.cluster, domains, pods)
	if err != nil {
		return nil, fmt.Errorf("tracking topology counts, %w", err)
	}

	// Calculate daemon overhead
	daemonOverhead, err := p.getDaemonOverhead(ctx, nodeTemplates)
	if err != nil {
		return nil, fmt.Errorf("getting daemon overhead, %w", err)
	}

	return scheduler.NewScheduler(ctx, p.kubeClient, nodeTemplates, provisionerList.Items, p.cluster, topology, instanceTypes, daemonOverhead, p.recorder).Solve(ctx, pods)
}

func (p *Provisioner) launch(ctx context.Context, node *scheduler.Node) error {
	// Check limits
	latest := &v1alpha5.Provisioner{}
	name := node.Requirements.Get(v1alpha5.ProvisionerNameLabelKey).Any()
	if err := p.kubeClient.Get(ctx, types.NamespacedName{Name: name}, latest); err != nil {
		return fmt.Errorf("getting current resource usage, %w", err)
	}
	if err := latest.Spec.Limits.ExceededBy(latest.Status.Resources); err != nil {
		return err
	}

	k8sNode, err := p.cloudProvider.Create(
		logging.WithLogger(ctx, logging.FromContext(ctx).Named("cloudprovider")),
		&cloudprovider.NodeRequest{InstanceTypeOptions: node.InstanceTypeOptions, Template: &node.NodeTemplate},
	)
	if err != nil {
		return fmt.Errorf("creating cloud provider instance, %w", err)
	}

	if err := mergo.Merge(k8sNode, node.ToNode()); err != nil {
		return fmt.Errorf("merging cloud provider node, %w", err)
	}
	// ensure we clear out the status
	k8sNode.Status = v1.NodeStatus{}

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
	for _, pod := range node.Pods {
		p.recorder.NominatePod(pod, k8sNode)
	}
	return nil
}

func (p *Provisioner) getDaemonOverhead(ctx context.Context, nodeTemplates []*scheduling.NodeTemplate) (map[*scheduling.NodeTemplate]v1.ResourceList, error) {
	overhead := map[*scheduling.NodeTemplate]v1.ResourceList{}

	daemonSetList := &appsv1.DaemonSetList{}
	if err := p.kubeClient.List(ctx, daemonSetList); err != nil {
		return nil, fmt.Errorf("listing daemonsets, %w", err)
	}

	for _, nodeTemplate := range nodeTemplates {
		var daemons []*v1.Pod
		for _, daemonSet := range daemonSetList.Items {
			p := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
			if err := nodeTemplate.Taints.Tolerates(p); err != nil {
				continue
			}
			if err := nodeTemplate.Requirements.Compatible(scheduling.NewPodRequirements(p)); err != nil {
				continue
			}
			daemons = append(daemons, p)
		}
		overhead[nodeTemplate] = resources.RequestsForPods(daemons...)
	}

	return overhead, nil
}

func (p *Provisioner) Validate(ctx context.Context, pod *v1.Pod) error {
	return multierr.Combine(
		validateAffinity(pod),
		p.volumeTopology.validatePersistentVolumeClaims(ctx, pod),
	)
}

func validateAffinity(p *v1.Pod) (errs error) {
	if p.Spec.Affinity == nil {
		return nil
	}
	if p.Spec.Affinity.NodeAffinity != nil {
		for _, term := range p.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			errs = multierr.Append(errs, validateNodeSelectorTerm(term.Preference))
		}
		if p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for _, term := range p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				errs = multierr.Append(errs, validateNodeSelectorTerm(term))
			}
		}
	}
	return errs
}

func validateNodeSelectorTerm(term v1.NodeSelectorTerm) (errs error) {
	if term.MatchFields != nil {
		errs = multierr.Append(errs, fmt.Errorf("node selector term with matchFields is not supported"))
	}
	if term.MatchExpressions != nil {
		for _, requirement := range term.MatchExpressions {
			if !v1alpha5.SupportedNodeSelectorOps.Has(string(requirement.Operator)) {
				errs = multierr.Append(errs, fmt.Errorf("node selector term has unsupported operator, %s", requirement.Operator))
			}
		}
	}
	return errs
}

var schedulingDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "allocation_controller",
		Name:      "scheduling_duration_seconds",
		Help:      "Duration of scheduling process in seconds. Broken down by provisioner and error.",
		Buckets:   metrics.DurationBuckets(),
	},
	[]string{metrics.ProvisionerLabel},
)

func init() {
	crmetrics.Registry.MustRegister(schedulingDuration)
}
