package drift

import (
	"context"
	"fmt"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	operatorcontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "drift"

type DriftController struct {
	kubeClient     client.Client
	cluster        *state.Cluster
	cloudProvider  *cloudprovider.CloudProvider
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cluster *state.Cluster, cloudProvider *cloudprovider.CloudProvider) *DriftController {
	return &DriftController{
		kubeClient:  kubeClient,
		cluster: cluster,
		cloudProvider: cloudProvider,
	}
}

func (r DriftController) Reconcile(ctx context.Context, request controllerruntime.Request) (reconcile.Result, error) {
	provisioner := &v1alpha5.Provisioner{}
	if err := r.kubeClient.Get(ctx, request.NamespacedName, provisioner); err != nil {
		return reconcile.Result{}, nil
	}

	nodes := v1.NodeList{}
	if err := r.kubeClient.List(ctx, &nodes, client.MatchingLabels{v1alpha5.ProvisionerNameLabelKey: request.Name}); err != nil {
		return reconcile.Result{}, err
	}

	driftedNodes := r.cloudProvider.GetDriftedNodes(ctx, provisioner, nodes.Items)
	for _, driftedNode := range driftedNodes {
		stored := driftedNode.DeepCopy()
		driftedNode.Annotations[v1alpha5.DriftedAnnotationKey] = "true"
		if err := r.kubeClient.Patch(ctx, &driftedNode, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patching node, %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (c DriftController) Builder(ctx context.Context, m manager.Manager) operatorcontroller.Builder {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha5.Provisioner{}).
		Watches(
			//Not sure if this is necessary
			&source.Kind{Type: &v1alpha1.AWSNodeTemplate{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				x := o.(*v1alpha1.AWSNodeTemplate)
				reconcileList :=  []reconcile.Request{}
				provisionerList := &v1alpha5.ProvisionerList{}
				err := c.kubeClient.List(ctx, provisionerList)
				if err != nil {
					//log
					return reconcileList
				}
				for _,provisioner := range provisionerList.Items {
					if  provisioner.Spec.ProviderRef != nil && provisioner.Spec.ProviderRef.Name ==  x.Name {
						reconcileList = append(reconcileList, reconcile.Request{NamespacedName: types.NamespacedName{Name:provisioner.GetName()}})
					}
				}
				return reconcileList
			}),
		).WithOptions(controller.Options{MaxConcurrentReconciles: 10})
}

func (c DriftController) LivenessProbe(_ *http.Request) error {
	return nil
}

