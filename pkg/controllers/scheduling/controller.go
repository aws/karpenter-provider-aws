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

package scheduling

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/controllers/provisioning"
	"github.com/awslabs/karpenter/pkg/utils/pod"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller for the resource
type Controller struct {
	kubeClient   client.Client
	provisioners *provisioning.Controller
	preferences  *Preferences
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, provisioners *provisioning.Controller) *Controller {
	return &Controller{
		kubeClient:   kubeClient,
		provisioners: provisioners,
		preferences:  NewPreferences(),
	}
}

// Reconcile the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("scheduling").With("pod", req.String()))
	pod := &v1.Pod{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, pod); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Ensure the pod can be provisioned
	if err := isUnschedulable(pod); err != nil {
		return reconcile.Result{}, nil
	}
	if err := validate(pod); err != nil {
		logging.FromContext(ctx).Debugf("Ignoring pod, %s", err.Error())
		return reconcile.Result{}, nil
	}
	// Schedule and requeue. If successful, will terminate in the unschedulable check above
	if err := c.Schedule(ctx, pod); err != nil {
		logging.FromContext(ctx).Errorf("Failed to schedule, %s", err.Error())
	}
	return reconcile.Result{Requeue: true}, nil
}

func (c *Controller) Schedule(ctx context.Context, pod *v1.Pod) error {
	// Relax preferences if pod has previously failed to schedule.
	c.preferences.Relax(ctx, pod)
	// Pick provisioner
	var provisioner *provisioning.Provisioner
	var errs error
	for _, candidate := range c.provisioners.List(ctx) {
		if err := candidate.Spec.DeepCopy().ValidatePod(pod); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tried provisioner/%s: %w", candidate.Name, err))
		} else {
			provisioner = candidate
			break
		}
	}
	if provisioner == nil {
		err := fmt.Errorf("matched 0/%d provisioners", len(multierr.Errors(errs)))
		if errs != nil {
			err = fmt.Errorf("%s, %w", err, errs)
		}
		return err
	}
	// Enqueue and wait for provisioning
	logging.FromContext(ctx).Debugf("Scheduling pod to provisioner %q", provisioner.Name)
	if err := provisioner.Add(ctx, pod); err != nil {
		return fmt.Errorf("provisioner %q failed", provisioner.Name)
	}
	return nil
}

func isUnschedulable(p *v1.Pod) error {
	if p.Spec.NodeName != "" {
		return fmt.Errorf("already scheduled")
	}
	if !pod.FailedToSchedule(p) {
		return fmt.Errorf("awaiting scheduling")
	}
	if pod.IsOwnedByDaemonSet(p) {
		return fmt.Errorf("owned by daemonset")
	}
	if pod.IsOwnedByNode(p) {
		return fmt.Errorf("owned by node")
	}
	return nil
}

func validate(p *v1.Pod) error {
	return multierr.Combine(
		validateAffinity(p),
		validateTopology(p),
	)
}

func validateTopology(pod *v1.Pod) (errs error) {
	for _, constraint := range pod.Spec.TopologySpreadConstraints {
		if supported := sets.NewString(v1.LabelHostname, v1.LabelTopologyZone); !supported.Has(constraint.TopologyKey) {
			errs = multierr.Append(errs, fmt.Errorf("unsupported topology key, %s not in %s", constraint.TopologyKey, supported))
		}
	}
	return errs
}

func validateAffinity(pod *v1.Pod) (errs error) {
	if pod.Spec.Affinity == nil {
		return nil
	}
	if pod.Spec.Affinity.PodAffinity != nil {
		errs = multierr.Append(errs, fmt.Errorf("pod affinity is not supported"))
	}
	if pod.Spec.Affinity.PodAntiAffinity != nil {
		errs = multierr.Append(errs, fmt.Errorf("pod anti-affinity is not supported"))
	}
	if pod.Spec.Affinity.NodeAffinity != nil {
		for _, term := range pod.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
			errs = multierr.Append(errs, validateNodeSelectorTerm(term.Preference))
		}
		if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
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
			if !sets.NewString(string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn)).Has(string(requirement.Operator)) {
				errs = multierr.Append(errs, fmt.Errorf("node selector term has unsupported operator, %s", requirement.Operator))
			}
		}
	}
	return errs
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("scheduling").
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10_000}).
		Complete(c)
}
