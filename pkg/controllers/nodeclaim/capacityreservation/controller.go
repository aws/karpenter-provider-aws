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

package capacityreservation

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/equality"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

// Controller is an nodeclaim capacity reservation controller.
type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

func NewController(
	kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider,
) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	cloudProviderNodeClaims, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing instances, %w", err)
	}

	providerIDCloudProviderNodeClaimMap := make(map[string]*karpv1.NodeClaim, len(cloudProviderNodeClaims))
	for _, cloudProviderNodeClaim := range cloudProviderNodeClaims {
		log.FromContext(ctx).WithValues("cloudProviderNodeClaim", cloudProviderNodeClaim).V(0).Info("reconcile nodeClaim adding cloudProviderNodeClaim")
		providerIDCloudProviderNodeClaimMap[cloudProviderNodeClaim.Status.ProviderID] = cloudProviderNodeClaim
	}

	nodeClaimList := &karpv1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, nodeClaimList); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims, %w", err)
	}

	// Find the NodeClaims that don't match
	// Then patch the label so that it adds or removes the capacity reservation label
	errs := make([]error, len(nodeClaimList.Items))
	for i := range nodeClaimList.Items {
		nodeClaim := nodeClaimList.Items[i]
		stored := nodeClaim.DeepCopy()

		log.FromContext(ctx).WithValues("nodeClaim", nodeClaim).V(0).Info("reconcile nodeClaim")

		cloudProviderNodeClaim, ok := providerIDCloudProviderNodeClaimMap[nodeClaim.Status.ProviderID]
		if !ok {
			continue
		}

		log.FromContext(ctx).WithValues("nodeClaim", nodeClaim, "cloudProviderNodeClaim", cloudProviderNodeClaim).V(0).Info("reconcile nodeClaim with cloudProviderNodeClaim")

		nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
			v1.LabelCapactiyReservationID: cloudProviderNodeClaim.Labels[v1.LabelCapactiyReservationID],
		})

		if !equality.Semantic.DeepEqual(stored, nodeClaim) {
			log.FromContext(ctx).WithValues("nodeClaim", nodeClaim, "stored", stored).V(0).Info("patch nodeClaim")
			if err := c.kubeClient.Patch(ctx, &nodeClaim, client.MergeFrom(stored)); err != nil {
				errs[i] = client.IgnoreNotFound(err)
			}
		}
	}

	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclaim.capacityreservation").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
