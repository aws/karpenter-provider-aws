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

package nodeclass

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	karpoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
	"sigs.k8s.io/karpenter/pkg/utils/result"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/awslabs/operatorpkg/reasonable"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

type Controller struct {
	kubeClient              client.Client
	recorder                events.Recorder
	region                  string
	launchTemplateProvider  launchtemplate.Provider
	instanceProfileProvider instanceprofile.Provider
	validation              *Validation
	reconcilers             []reconcile.TypedReconciler[*v1.EC2NodeClass]
}

func NewController(
	clk clock.Clock,
	kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider,
	recorder events.Recorder,
	region string,
	subnetProvider subnet.Provider,
	securityGroupProvider securitygroup.Provider,
	amiProvider amifamily.Provider,
	instanceProfileProvider instanceprofile.Provider,
	instanceTypeProvider instancetype.Provider,
	launchTemplateProvider launchtemplate.Provider,
	capacityReservationProvider capacityreservation.Provider,
	ec2api sdk.EC2API,
	validationCache *cache.Cache,
	amiResolver amifamily.Resolver,
) *Controller {
	validation := NewValidationReconciler(kubeClient, cloudProvider, ec2api, amiResolver, instanceTypeProvider, launchTemplateProvider, validationCache)
	return &Controller{
		kubeClient:              kubeClient,
		recorder:                recorder,
		region:                  region,
		launchTemplateProvider:  launchTemplateProvider,
		instanceProfileProvider: instanceProfileProvider,
		validation:              validation,
		reconcilers: []reconcile.TypedReconciler[*v1.EC2NodeClass]{
			NewAMIReconciler(amiProvider),
			NewCapacityReservationReconciler(clk, capacityReservationProvider),
			NewSubnetReconciler(subnetProvider),
			NewSecurityGroupReconciler(securityGroupProvider),
			NewInstanceProfileReconciler(instanceProfileProvider, region),
			validation,
			NewReadinessReconciler(launchTemplateProvider),
		},
		//iamapi: iamapi,
	}
}

func (c *Controller) Name() string {
	return "nodeclass"
}

//nolint:gocyclo
func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())

	if !nodeClass.GetDeletionTimestamp().IsZero() {
		return c.finalize(ctx, nodeClass)
	}

	if !controllerutil.ContainsFinalizer(nodeClass, v1.TerminationFinalizer) {
		stored := nodeClass.DeepCopy()
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)

		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the finalizer list
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); client.IgnoreNotFound(err) != nil {
			if errors.IsConflict(err) {
				log.Printf("REQUEUEING 1, conflict error: %s", err)
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, err
		}
	}
	stored := nodeClass.DeepCopy()

	var results []reconcile.Result
	var errs error
	for _, reconciler := range c.reconcilers {
		if _, ok := reconciler.(*CapacityReservation); ok && !karpoptions.FromContext(ctx).FeatureGates.ReservedCapacity {
			continue
		}
		res, err := reconciler.Reconcile(ctx, nodeClass)
		errs = multierr.Append(errs, err)
		results = append(results, res)
	}

	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the status condition list
		if err := c.kubeClient.Status().Patch(ctx, nodeClass, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			if errors.IsConflict(err) {
				// Delete the created instance profile if it hasn't yet patched into status
				// This prevents multiple instance profiles from being created for the same role
				if nodeClass.Spec.Role != "" && nodeClass.Status.InstanceProfile != "" {
					if profile, err := c.instanceProfileProvider.Get(ctx, nodeClass.Status.InstanceProfile); err == nil {
						if len(profile.Roles) > 0 {
							currentRole := lo.FromPtr(profile.Roles[0].RoleName)
							if currentRole == nodeClass.Spec.Role {
								if err := c.instanceProfileProvider.Delete(ctx, nodeClass.Status.InstanceProfile); err != nil {
									return reconcile.Result{}, fmt.Errorf("deleting instance profile, %w", err)
								}
							}
						}
					}
				}
				log.Printf("REQUEUEING 2, conflict error: %s", err)
				return reconcile.Result{Requeue: true}, nil
			}
			errs = multierr.Append(errs, client.IgnoreNotFound(err))
		}
	}
	if errs != nil {
		return reconcile.Result{}, errs
	}
	return result.Min(results...), nil
}

func (c *Controller) cleanupInstanceProfiles(ctx context.Context, nodeClass *v1.EC2NodeClass) error {
	if nodeClass.Spec.Role == "" {
		return nil
	}

	out, err := c.instanceProfileProvider.ListByPrefix(ctx, fmt.Sprintf("/karpenter/%s/%s/", options.FromContext(ctx).ClusterName, string(nodeClass.UID)))
	if err != nil {
		return fmt.Errorf("listing instance profiles, %w", err)
	}

	for _, profile := range out {
		if err := c.cleanupSingleProfile(ctx, profile); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) cleanupSingleProfile(ctx context.Context, profile *iamtypes.InstanceProfile) error {
	name := *profile.InstanceProfileName

	if err := c.instanceProfileProvider.Delete(ctx, name); err != nil {
		return fmt.Errorf("deleting instance profile, %w", err)
	}
	return nil
}

func (c *Controller) finalize(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	stored := nodeClass.DeepCopy()
	if !controllerutil.ContainsFinalizer(nodeClass, v1.TerminationFinalizer) {
		return reconcile.Result{}, nil
	}
	nodeClaims := &karpv1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, nodeClaims, nodeclaimutils.ForNodeClass(nodeClass)); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims that are using nodeclass, %w", err)
	}
	if len(nodeClaims.Items) > 0 {
		c.recorder.Publish(WaitingOnNodeClaimTerminationEvent(nodeClass, lo.Map(nodeClaims.Items, func(nc karpv1.NodeClaim, _ int) string { return nc.Name })))
		return reconcile.Result{RequeueAfter: time.Minute * 10}, nil // periodically fire the event
	}
	if err := c.cleanupInstanceProfiles(ctx, nodeClass); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.launchTemplateProvider.DeleteAll(ctx, nodeClass); err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting launch templates, %w", err)
	}
	controllerutil.RemoveFinalizer(nodeClass, v1.TerminationFinalizer)
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the finalizer list
		// https://github.com/kubernetes/kubernetes/issues/111643#issuecomment-2016489732
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, client.IgnoreNotFound(fmt.Errorf("removing termination finalizer, %w", err))
		}
	}
	c.validation.clearCacheEntries(nodeClass)
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		For(&v1.EC2NodeClass{}).
		Watches(
			&karpv1.NodeClaim{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
				nc := o.(*karpv1.NodeClaim)
				if nc.Spec.NodeClassRef == nil {
					return nil
				}
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: nc.Spec.NodeClassRef.Name}}}
			}),
			// Watch for NodeClaim deletion events
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return false },
				UpdateFunc: func(e event.UpdateEvent) bool { return false },
				DeleteFunc: func(e event.DeleteEvent) bool { return true },
			}),
		).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 10,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
