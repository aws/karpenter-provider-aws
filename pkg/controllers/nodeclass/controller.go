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
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
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

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/events"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/instanceprofile"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"
)

type Controller struct {
	kubeClient              client.Client
	recorder                events.Recorder
	subnetProvider          *subnet.Provider
	securityGroupProvider   *securitygroup.Provider
	amiProvider             *amifamily.Provider
	instanceProfileProvider *instanceprofile.Provider
}

func NewController(kubeClient client.Client, recorder events.Recorder, subnetProvider *subnet.Provider, securityGroupProvider *securitygroup.Provider,
	amiProvider *amifamily.Provider, instanceProfileProvider *instanceprofile.Provider) *Controller {
	return &Controller{
		kubeClient:              kubeClient,
		recorder:                recorder,
		subnetProvider:          subnetProvider,
		securityGroupProvider:   securityGroupProvider,
		amiProvider:             amiProvider,
		instanceProfileProvider: instanceProfileProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	stored := nodeClass.DeepCopy()
	if !nodeClass.IsNodeTemplate {
		controllerutil.AddFinalizer(nodeClass, v1beta1.TerminationFinalizer)
	}
	if nodeClass.Annotations[v1beta1.AnnotationEC2NodeClassHashVersion] != v1beta1.EC2NodeClassHashVersion {
		if err := c.updateNodeClaimHash(ctx, nodeClass); err != nil {
			return reconcile.Result{}, err
		}
	}
	nodeClass.Annotations = lo.Assign(nodeClass.Annotations, nodeclassutil.HashAnnotation(nodeClass))

	err := multierr.Combine(
		c.resolveSubnets(ctx, nodeClass),
		c.resolveSecurityGroups(ctx, nodeClass),
		c.resolveAMIs(ctx, nodeClass),
		c.resolveInstanceProfile(ctx, nodeClass),
	)
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		statusCopy := nodeClass.DeepCopy()
		if patchErr := nodeclassutil.Patch(ctx, c.kubeClient, stored, nodeClass); patchErr != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
		if patchErr := nodeclassutil.PatchStatus(ctx, c.kubeClient, stored, statusCopy); patchErr != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
	}
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Finalize(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	stored := nodeClass.DeepCopy()
	if nodeClass.IsNodeTemplate {
		return reconcile.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(nodeClass, v1beta1.TerminationFinalizer) {
		return reconcile.Result{}, nil
	}
	nodeClaimList := &corev1beta1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, nodeClaimList, client.MatchingFields{"spec.nodeClassRef.name": nodeClass.Name}); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims that are using nodeclass, %w", err)
	}
	if len(nodeClaimList.Items) > 0 {
		c.recorder.Publish(WaitingOnNodeClaimTerminationEvent(nodeClass, lo.Map(nodeClaimList.Items, func(nc corev1beta1.NodeClaim, _ int) string { return nc.Name })))
		return reconcile.Result{RequeueAfter: time.Minute * 10}, nil // periodically fire the event
	}
	if nodeClass.Spec.Role != "" {
		if err := c.instanceProfileProvider.Delete(ctx, nodeClass); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting instance profile, %w", err)
		}
	}
	controllerutil.RemoveFinalizer(nodeClass, v1beta1.TerminationFinalizer)
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		if err := nodeclassutil.Patch(ctx, c.kubeClient, stored, nodeClass); err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(fmt.Errorf("removing termination finalizer, %w", err))
		}
	}
	return reconcile.Result{}, nil
}

func (c *Controller) resolveSubnets(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	subnets, err := c.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return err
	}
	if len(subnets) == 0 {
		nodeClass.Status.Subnets = nil
		return fmt.Errorf("no subnets exist given constraints %v", nodeClass.Spec.SubnetSelectorTerms)
	}
	sort.Slice(subnets, func(i, j int) bool {
		if int(*subnets[i].AvailableIpAddressCount) != int(*subnets[j].AvailableIpAddressCount) {
			return int(*subnets[i].AvailableIpAddressCount) > int(*subnets[j].AvailableIpAddressCount)
		}
		return *subnets[i].SubnetId < *subnets[j].SubnetId
	})
	nodeClass.Status.Subnets = lo.Map(subnets, func(ec2subnet *ec2.Subnet, _ int) v1beta1.Subnet {
		return v1beta1.Subnet{
			ID:   *ec2subnet.SubnetId,
			Zone: *ec2subnet.AvailabilityZone,
		}
	})
	return nil
}

func (c *Controller) resolveSecurityGroups(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	securityGroups, err := c.securityGroupProvider.List(ctx, nodeClass)
	if err != nil {
		return err
	}
	if len(securityGroups) == 0 && len(nodeClass.Spec.SecurityGroupSelectorTerms) > 0 {
		nodeClass.Status.SecurityGroups = nil
		return fmt.Errorf("no security groups exist given constraints")
	}
	sort.Slice(securityGroups, func(i, j int) bool {
		return *securityGroups[i].GroupId < *securityGroups[j].GroupId
	})
	nodeClass.Status.SecurityGroups = lo.Map(securityGroups, func(securityGroup *ec2.SecurityGroup, _ int) v1beta1.SecurityGroup {
		return v1beta1.SecurityGroup{
			ID:   *securityGroup.GroupId,
			Name: *securityGroup.GroupName,
		}
	})
	return nil
}

func (c *Controller) resolveAMIs(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	amis, err := c.amiProvider.Get(ctx, nodeClass, &amifamily.Options{})
	if err != nil {
		return err
	}
	if len(amis) == 0 {
		nodeClass.Status.AMIs = nil
		return fmt.Errorf("no amis exist given constraints")
	}
	nodeClass.Status.AMIs = lo.Map(amis, func(ami amifamily.AMI, _ int) v1beta1.AMI {
		reqs := ami.Requirements.NodeSelectorRequirements()
		sort.Slice(reqs, func(i, j int) bool {
			if len(reqs[i].Key) != len(reqs[j].Key) {
				return len(reqs[i].Key) < len(reqs[j].Key)
			}
			return reqs[i].Key < reqs[j].Key
		})
		return v1beta1.AMI{
			Name:         ami.Name,
			ID:           ami.AmiID,
			Requirements: reqs,
		}
	})

	return nil
}

func (c *Controller) resolveInstanceProfile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	if nodeClass.IsNodeTemplate {
		return nil
	}
	if nodeClass.Spec.Role != "" {
		name, err := c.instanceProfileProvider.Create(ctx, nodeClass)
		if err != nil {
			return fmt.Errorf("creating instance profile, %w", err)
		}
		nodeClass.Status.InstanceProfile = name
	} else {
		nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
	}
	return nil
}

// Updating `ec2nodeclass-hash-version` annotation inside the karpenter controller means a breaking change has been made to the hash calculation.
// `ec2nodeclass-hash` annotation on the EC2NodeClass will be updated, due to the breaking change, making the `ec2nodeclass-hash` on the NodeClaim different from
// EC2NodeClass. Since, we cannot rely on the `ec2nodeclass-hash` on the NodeClaims, due to the breaking change, we will need to re-calculate the hash and update the annotation.
// For more information on the Drift Hash Versioning: https://github.com/kubernetes-sigs/karpenter/blob/main/designs/drift-hash-versioning.md
func (c *Controller) updateNodeClaimHash(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) error {
	if nodeClass.IsNodeTemplate {
		return nil
	}

	ncList := &corev1beta1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, ncList, client.MatchingFields{"spec.nodeClassRef.name": nodeClass.Name}); err != nil {
		return err
	}

	errs := make([]error, len(ncList.Items))
	for i := range ncList.Items {
		nc := ncList.Items[i]
		stored := nc.DeepCopy()

		if nc.Annotations[v1beta1.AnnotationEC2NodeClassHashVersion] != v1beta1.EC2NodeClassHashVersion {
			nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
				v1beta1.AnnotationEC2NodeClassHashVersion: v1beta1.EC2NodeClassHashVersion,
			})

			// Any NodeClaim that is already drifted will remain drifted if the karpenter.k8s.aws/nodepool-hash-version doesn't match
			// Since the hashing mechanism has changed we will not be able to determine if the drifted status of the NodeClaim has changed
			if nc.StatusConditions().GetCondition(corev1beta1.Drifted) == nil {
				nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
					v1beta1.AnnotationEC2NodeClassHash: nodeClass.Hash(),
				})
			}

			if !equality.Semantic.DeepEqual(stored, nc) {
				if err := c.kubeClient.Patch(ctx, &nc, client.MergeFrom(stored)); err != nil {
					errs[i] = client.IgnoreNotFound(err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

var _ corecontroller.FinalizingTypedController[*v1beta1.EC2NodeClass] = (*NodeClassController)(nil)

//nolint:revive
type NodeClassController struct {
	*Controller
}

func NewNodeClassController(kubeClient client.Client, recorder events.Recorder, subnetProvider *subnet.Provider, securityGroupProvider *securitygroup.Provider,
	amiProvider *amifamily.Provider, instanceProfileProvider *instanceprofile.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1beta1.EC2NodeClass](kubeClient, &NodeClassController{
		Controller: NewController(kubeClient, recorder, subnetProvider, securityGroupProvider, amiProvider, instanceProfileProvider),
	})
}

func (c *Controller) Name() string {
	return "nodeclass"
}

func (c *NodeClassController) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1beta1.EC2NodeClass{}).
		Watches(
			&corev1beta1.NodeClaim{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
				nc := o.(*corev1beta1.NodeClaim)
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
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 1*time.Minute),
				// 10 qps, 100 bucket size
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			MaxConcurrentReconciles: 10,
		}))
}

type NodeTemplateController struct {
	*Controller
}

func NewNodeTemplateController(kubeClient client.Client, recorder events.Recorder, subnetProvider *subnet.Provider, securityGroupProvider *securitygroup.Provider,
	amiProvider *amifamily.Provider, instanceProfileProvider *instanceprofile.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1alpha1.AWSNodeTemplate](kubeClient, &NodeTemplateController{
		Controller: NewController(kubeClient, recorder, subnetProvider, securityGroupProvider, amiProvider, instanceProfileProvider),
	})
}

func (c *NodeTemplateController) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	return c.Controller.Reconcile(ctx, nodeclassutil.New(nodeTemplate))
}

func (c *NodeTemplateController) Name() string {
	return "awsnodetemplate"
}

func (c *NodeTemplateController) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1alpha1.AWSNodeTemplate{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 1*time.Minute),
				// 10 qps, 100 bucket size
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			MaxConcurrentReconciles: 10,
		}))
}
