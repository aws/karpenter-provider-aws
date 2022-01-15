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

package selection

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/utils/pod"
	"github.com/go-logr/zapr"
	"go.uber.org/multierr"
	"go.uber.org/zap"
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

const controllerName = "selection"

// Controller for the resource
type Controller struct {
	kubeClient     client.Client
	provisioners   *provisioning.Controller
	preferences    *Preferences
	volumeTopology *VolumeTopology
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, provisioners *provisioning.Controller) *Controller {
	return &Controller{
		kubeClient:     kubeClient,
		provisioners:   provisioners,
		preferences:    NewPreferences(),
		volumeTopology: NewVolumeTopology(kubeClient),
	}
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
		logging.FromContext(ctx).Debugf("Ignoring pod, %s", err.Error())
		return reconcile.Result{}, nil
	}
	// Select a provisioner, wait for it to bind the pod, and verify scheduling succeeded in the next loop
	if err := c.selectProvisioner(ctx, pod); err != nil {
		logging.FromContext(ctx).Debugf("Could not schedule pod, %s", err.Error())
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: time.Second * 5}, nil
}

func (c *Controller) selectProvisioner(ctx context.Context, pod *v1.Pod) (errs error) {
	// Relax preferences if pod has previously failed to schedule.
	c.preferences.Relax(ctx, pod)
	// Inject volume topological requirements
	if err := c.volumeTopology.Inject(ctx, pod); err != nil {
		return fmt.Errorf("getting volume topology requirements, %w", err)
	}
	// Pick provisioner
	var provisioner *provisioning.Provisioner
	provisioners := c.provisioners.List(ctx)
	if len(provisioners) == 0 {
		return nil
	}
	for _, candidate := range c.provisioners.List(ctx) {

		if err := candidate.Spec.DeepCopy().ValidatePod(pod); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("tried provisioner/%s: %w", candidate.Name, err))
		} else {
			provisioner = candidate
			break
		}
	}
	if provisioner == nil {
		return fmt.Errorf("matched 0/%d provisioners, %w", len(multierr.Errors(errs)), errs)
	}
	select {
	case <-provisioner.Add(pod):
	case <-ctx.Done():
	}
	return nil
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
		Named(controllerName).
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10_000}).
		WithLogger(zapr.NewLogger(zap.NewNop())).
		Complete(c)
}
