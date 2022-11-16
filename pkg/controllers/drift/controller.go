package drift

import (
	"context"
	"fmt"
	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	operatorcontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
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
	node := &v1.Node{}
	if err := d.kubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		return reconcile.Result{}, nil
	}

	if _, drifted := node.Annotations[v1alpha5.DriftedAnnotationKey]; drifted {
		return reconcile.Result{}, nil
	}

	provisionerName, provisionerExists := node.Labels[v1alpha5.ProvisionerNameLabelKey]
	if !provisionerExists {
		//Only karpenter owned Nodes.
		return reconcile.Result{}, nil
	}

	provisioner := &v1alpha5.Provisioner{}
	if err := d.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		return reconcile.Result{}, nil
	}

	drifted := d.cloudProvider.IsNodeDrifted(ctx, provisioner, *node)
	if drifted {
		stored := node.DeepCopy()
		node.Annotations[v1alpha5.DriftedAnnotationKey] = "true"
		if err := d.kubeClient.Patch(ctx, node, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patching node, %w", err)
		}
	}

	return reconcile.Result{RequeueAfter: 30 * time.Minute}, nil
}

func (d Drift) Builder(ctx context.Context, m manager.Manager) operatorcontroller.Builder {
	if err := m.GetFieldIndexer().IndexField(ctx, &v1alpha5.Provisioner{}, ".spec.providerRef.name", func(rawObj client.Object) []string {
		provisioner := rawObj.(*v1alpha5.Provisioner)
		return []string{provisioner.Spec.ProviderRef.Name}
	}); err != nil {
		//todo:Need inputs on this, or this indexer needs to be put somewhere else or removed altogether
	}

	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1.Node{}).
		Watches(
			&source.Kind{Type: &v1alpha5.Provisioner{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				provisioner := o.(*v1alpha5.Provisioner)
				if provisioner.Spec.ProviderRef == nil {
					return requests
				}
				// Ensure provisioner has a defined AWSNodeTemplate
				nodeTemplate := &v1alpha1.AWSNodeTemplate{}
				if err := d.kubeClient.Get(ctx, types.NamespacedName{Name: provisioner.Spec.ProviderRef.Name}, nodeTemplate); err != nil {
					logging.FromContext(ctx).Errorf("Failed to get AWSNodeTemplates when mapping drift watch events, %s", err)
					return requests
				}
				nodes := &v1.NodeList{}
				if err := d.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})); err != nil {
					logging.FromContext(ctx).Errorf("Failed to list nodes when mapping drift watch events, %s", err)
					return requests
				}
				for _, node := range nodes.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
				}
				return requests
			})).
		Watches(
			&source.Kind{Type: &v1alpha1.AWSNodeTemplate{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				provisioners := &v1alpha5.ProvisionerList{}
				//Indexer is needed for this.
				if err := d.kubeClient.List(ctx, provisioners, client.MatchingFields{".spec.providerRef.name": o.GetName()}); err != nil {
					//log
					logging.FromContext(ctx).Errorf("listing provisioners for AWSNodeTemplate reconciliation %w", err)
					return requests
				}
				for _, provisioner := range provisioners.Items {
					nodes := &v1.NodeList{}
					if err := d.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})); err != nil {
						logging.FromContext(ctx).Errorf("Failed to list nodes when mapping drift watch events, %s", err)
						return requests
					}
					for _, node := range nodes.Items {
						requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
					}
				}
				return requests
			}),
		).WithOptions(controller.Options{MaxConcurrentReconciles: 10})
}

func (d Drift) LivenessProbe(_ *http.Request) error {
	return nil
}
