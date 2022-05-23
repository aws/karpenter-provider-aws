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
	"time"

	"github.com/aws/karpenter/pkg/events"

	"github.com/aws/karpenter/pkg/controllers/state"

	"github.com/aws/karpenter/pkg/cloudprovider"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/pod"
)

const controllerName = "provisioning"

// Controller for the resource
type Controller struct {
	kubeClient  client.Client
	provisioner *Provisioner
	recorder    events.Recorder
}

// NewController constructs a controller instance
func NewController(ctx context.Context, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, recorder events.Recorder, cloudProvider cloudprovider.CloudProvider, cluster *state.Cluster) *Controller {
	return &Controller{
		kubeClient:  kubeClient,
		provisioner: NewProvisioner(ctx, kubeClient, coreV1Client, recorder, cloudProvider, cluster),
		recorder:    recorder,
	}
}

func (c *Controller) Recorder() events.Recorder {
	return c.recorder
}

// Reconcile the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("pod", req.String()))
	pod := &v1.Pod{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, pod); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Ensure the pod can be provisioned
	if !isProvisionable(pod) {
		return reconcile.Result{}, nil
	}
	if err := validate(pod); err != nil {
		return reconcile.Result{}, nil
	}
	// Ensure pvcs exist if requested by a pod
	if err := c.provisioner.volumeTopology.validatePersistentVolumeClaims(ctx, pod); err != nil {
		return reconcile.Result{}, nil
	}
	// Enqueue to the provisioner
	c.provisioner.Trigger()
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

// Deprecated: TriggerAndWait is used for unit testing purposes only
func (c *Controller) TriggerAndWait() {
	c.provisioner.TriggerAndWait()
}

func isProvisionable(p *v1.Pod) bool {
	return !pod.IsScheduled(p) &&
		!pod.IsPreempting(p) &&
		pod.FailedToSchedule(p) &&
		!pod.IsOwnedByDaemonSet(p) &&
		!pod.IsOwnedByNode(p)
}

func validate(p *v1.Pod) error {
	return multierr.Combine(
		validateAffinity(p),
		validateTopology(p),
	)
}

func validateTopology(pod *v1.Pod) (errs error) {
	for _, constraint := range pod.Spec.TopologySpreadConstraints {
		if !v1alpha5.ValidTopologyKeys.Has(constraint.TopologyKey) {
			errs = multierr.Append(errs, fmt.Errorf("unsupported topology spread constraint key, %s not in %s", constraint.TopologyKey, v1alpha5.ValidTopologyKeys))
		}
	}
	return errs
}

func validateAffinity(p *v1.Pod) (errs error) {
	if p.Spec.Affinity == nil {
		return nil
	}
	if p.Spec.Affinity.PodAffinity != nil {
		for _, term := range p.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
			errs = multierr.Append(errs, validatePodAffinityTerm(term))
		}
		for _, term := range p.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			errs = multierr.Append(errs, validatePodAffinityTerm(term.PodAffinityTerm))
		}
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

func validatePodAffinityTerm(term v1.PodAffinityTerm) error {
	if !v1alpha5.ValidTopologyKeys.Has(term.TopologyKey) {
		return fmt.Errorf("unsupported topology key in pod affinity, %s not in %s", term.TopologyKey, v1alpha5.ValidTopologyKeys)
	}
	return nil
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

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}
