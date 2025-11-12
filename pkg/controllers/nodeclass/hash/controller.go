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

package hash

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/equality"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	"github.com/awslabs/operatorpkg/reasonable"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type Controller struct {
	kubeClient client.Client
	region     string
}

func NewController(kubeClient client.Client, region string) *Controller {
	return &Controller{
		kubeClient: kubeClient,
		region:     region,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclass.hash")

	stored := nodeClass.DeepCopy()

	if nodeClass.Annotations[v1.AnnotationEC2NodeClassHashVersion] != v1.EC2NodeClassHashVersion {
		if err := c.updateNodeClaimHash(ctx, nodeClass); err != nil {
			return reconcile.Result{}, err
		}
	}
	nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
		v1.AnnotationEC2NodeClassHash:        nodeClass.HashForRegion(c.region),
		v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
	})

	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclass.hash").
		For(&v1.EC2NodeClass{}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 10,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

// Updating `ec2nodeclass-hash-version` annotation inside the karpenter controller means a breaking change has been made to the hash calculation.
// `ec2nodeclass-hash` annotation on the EC2NodeClass will be updated, due to the breaking change, making the `ec2nodeclass-hash` on the NodeClaim different from
// EC2NodeClass. Since, we cannot rely on the `ec2nodeclass-hash` on the NodeClaims, due to the breaking change, we will need to re-calculate the hash and update the annotation.
// For more information on the Drift Hash Versioning: https://github.com/kubernetes-sigs/karpenter/blob/main/designs/drift-hash-versioning.md
func (c *Controller) updateNodeClaimHash(ctx context.Context, nodeClass *v1.EC2NodeClass) error {
	nodeClaims := &karpv1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, nodeClaims, nodeclaimutils.ForNodeClass(nodeClass)); err != nil {
		return err
	}

	errs := make([]error, len(nodeClaims.Items))
	for i := range nodeClaims.Items {
		nc := &nodeClaims.Items[i]
		stored := nc.DeepCopy()

		if nc.Annotations[v1.AnnotationEC2NodeClassHashVersion] != v1.EC2NodeClassHashVersion {
			nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
				v1.AnnotationEC2NodeClassHashVersion: v1.EC2NodeClassHashVersion,
			})

			// Any NodeClaim that is already drifted will remain drifted if the karpenter.k8s.aws/nodepool-hash-version doesn't match
			// Since the hashing mechanism has changed we will not be able to determine if the drifted status of the NodeClaim has changed
			if nc.StatusConditions().Get(karpv1.ConditionTypeDrifted) == nil {
				nc.Annotations = lo.Assign(nc.Annotations, map[string]string{
					v1.AnnotationEC2NodeClassHash: nodeClass.HashForRegion(c.region),
				})
			}

			if !equality.Semantic.DeepEqual(stored, nc) {
				if err := c.kubeClient.Patch(ctx, nc, client.MergeFrom(stored)); err != nil {
					errs[i] = client.IgnoreNotFound(err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}
