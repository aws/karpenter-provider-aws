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

var _ operatorcontroller.TypedController[*v1.Node] = (*Drift)(nil)

type Drift struct {
	kubeClient    client.Client
	cloudProvider *cloudprovider.CloudProvider
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cloudProvider *cloudprovider.CloudProvider) operatorcontroller.Controller {
	return operatorcontroller.Typed[*v1.Node](kubeClient, &Drift{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	})
}

func (c *Drift) Name() string {
	return "drift"
}

func (d Drift) Reconcile(ctx context.Context, node *v1.Node) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(d.Name()).With("node", node.Name))

	provisionerName, provisionerExists := node.Labels[v1alpha5.ProvisionerNameLabelKey]
	if !provisionerExists {
		return reconcile.Result{}, nil
	}

	if drifted, ok := node.Labels[v1alpha5.DriftedLabelKey]; ok && drifted == "true" {
		return reconcile.Result{}, nil
	}

	provisioner := &v1alpha5.Provisioner{}
	if err := d.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("getting provisioner, %w", err)
	}

	if drifted, err := d.cloudProvider.IsNodeDrifted(ctx, provisioner, node); err != nil {
		return reconcile.Result{}, fmt.Errorf("getting drift for node, %w", err)
	} else if drifted {
		stored := node.DeepCopy()
		node.Labels[v1alpha5.DriftedLabelKey] = "true"
		if err := d.kubeClient.Patch(ctx, node, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patching node, %w", err)
		}
	}

	return reconcile.Result{RequeueAfter: 30 * time.Minute}, nil
}

func (d Drift) Builder(ctx context.Context, m manager.Manager) operatorcontroller.Builder {
	builder := controllerruntime.
		NewControllerManagedBy(m).
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
					logging.FromContext(ctx).Errorf("getting AWSNodeTemplates when mapping drift watch events, %s", err)
					return requests
				}
				nodes := &v1.NodeList{}
				if err := d.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})); err != nil {
					logging.FromContext(ctx).Errorf("listing nodes when mapping drift watch events, %s", err)
					return requests
				}
				for _, node := range nodes.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
				}
				return requests
			})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10})

	if err := m.GetFieldIndexer().IndexField(ctx, &v1alpha5.Provisioner{}, ".spec.providerRef.name", func(rawObj client.Object) []string {
		provisioner := rawObj.(*v1alpha5.Provisioner)
		return []string{provisioner.Spec.ProviderRef.Name}
	}); err != nil {
		//Return early, controller won't be able to get provisioners related to AWSNodeTemplate if the index field failed
		logging.FromContext(ctx).Errorf("creating index for provisioner while building drift controller, %w", err)
		return operatorcontroller.Adapt(builder)
	}

	return operatorcontroller.Adapt(builder.Watches(
		&source.Kind{Type: &v1alpha1.AWSNodeTemplate{}},
		handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
			provisioners := &v1alpha5.ProvisionerList{}
			if err := d.kubeClient.List(ctx, provisioners, client.MatchingFields{".spec.providerRef.name": o.GetName()}); err != nil {
				logging.FromContext(ctx).Errorf("listing provisioners for AWSNodeTemplate reconciliation %w", err)
				return requests
			}
			for _, provisioner := range provisioners.Items {
				nodes := &v1.NodeList{}
				if err := d.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})); err != nil {
					logging.FromContext(ctx).Errorf("listing nodes when mapping drift watch events, %s", err)
					return requests
				}
				for _, node := range nodes.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
				}
			}
			return requests
		}),
	))
}

func (d Drift) LivenessProbe(_ *http.Request) error {
	return nil
}
