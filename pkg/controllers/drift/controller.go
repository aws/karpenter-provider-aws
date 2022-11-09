package drift

import (
	"context"
	"fmt"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	operatorcontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	"net/http"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const controllerName = "drift"

type Drift struct {
	kubeClient    client.Client
	cloudProvider *cloudprovider.CloudProvider
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cloudProvider *cloudprovider.CloudProvider) *Drift {
	return &Drift{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

func (d Drift) Reconcile(ctx context.Context, req controllerruntime.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("provisioner", req.Name))

	provisioner := &v1alpha5.Provisioner{}
	if err := d.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		return reconcile.Result{}, nil
	}

	nodes := v1.NodeList{}
	if err := d.kubeClient.List(ctx, &nodes, client.MatchingLabels{v1alpha5.ProvisionerNameLabelKey: req.Name}); err != nil {
		return reconcile.Result{}, err
	}

	alreadyDriftedNodes := lo.Filter(nodes.Items, func(node v1.Node, _ int) bool {
		_, drifted := node.Annotations[v1alpha5.DriftedAnnotationKey]
		return drifted
	})

	if len(alreadyDriftedNodes) == len(nodes.Items) {
		return reconcile.Result{}, nil
	}

	driftedNodes := d.cloudProvider.GetDriftedNodes(ctx, provisioner, nodes.Items)
	for _, driftedNode := range driftedNodes {
		//Check for already drifted node and continue
		if _, ok := driftedNode.Annotations[v1alpha5.DriftedAnnotationKey]; ok {
			continue
		}

		stored := driftedNode.DeepCopy()
		driftedNode.Annotations[v1alpha5.DriftedAnnotationKey] = "true"
		if err := d.kubeClient.Patch(ctx, &driftedNode, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patching node, %w", err)
		}
		logging.FromContext(ctx).Infof("Marked node %s as Drifted", driftedNode.Name)
	}

	return reconcile.Result{RequeueAfter: 30 * time.Minute}, nil
}

func (d Drift) Builder(ctx context.Context, m manager.Manager) operatorcontroller.Builder {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha5.Provisioner{}).
		Watches(
			//Todo:RemoveComment: Not sure if this is necessary, or if primary watch should be AwsNodeTemplate,
			//but I think its better to reconcile Provisioner as getInstanceType in cloudprovider is deeply rooted
			//in provisioner, so we do need the reference of it, even if we primarily reconcile AwsNodeTemplate
			&source.Kind{Type: &v1alpha1.AWSNodeTemplate{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				reconcileList := []reconcile.Request{}
				x, ok := o.(*v1alpha1.AWSNodeTemplate)
				if !ok {
					logging.FromContext(ctx).Debugf("Not able to cast AWSNodeTemplate in EnqueueRequestsFromMapFunc")
					return reconcileList
				}
				provisionerList := &v1alpha5.ProvisionerList{}
				err := d.kubeClient.List(ctx, provisionerList)
				if err != nil {
					//log
					logging.FromContext(ctx).Errorf("listing provisioners for AWSNodeTemplate reconciliation %w", err)
					return reconcileList
				}
				for _, provisioner := range provisionerList.Items {
					if provisioner.Spec.ProviderRef != nil && provisioner.Spec.ProviderRef.Name == x.Name {
						reconcileList = append(reconcileList, reconcile.Request{NamespacedName: types.NamespacedName{Name: provisioner.GetName()}})
					}
				}
				return reconcileList
			}),
		).WithOptions(controller.Options{MaxConcurrentReconciles: 10})
}

func (d Drift) LivenessProbe(_ *http.Request) error {
	return nil
}
