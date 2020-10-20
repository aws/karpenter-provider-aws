package controllers

import (
	"context"

	"github.com/ellistarn/karpenter/pkg/apis"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apis.AddToScheme(scheme)
}

type Manager struct {
	manager.Manager
}

// NewManager instantiates a controller manager or panics
func NewManager(config *rest.Config, options controllerruntime.Options) Manager {
	manager, err := controllerruntime.NewManager(config, options)
	Expect(err).ToNot(HaveOccurred(), "Failed to create controller manager")
	Expect(
		manager.GetFieldIndexer().IndexField(
			context.Background(),
			&v1.Pod{},
			"spec.nodeName",
			podSchedulingIndex,
		),
	).To(Succeed(), "Failed to setup pod indexer")
	return Manager{Manager: manager}
}

func (m *Manager) Register(controllers ...Controller) {
	for _, controller := range controllers {
		var builder = controllerruntime.NewControllerManagedBy(m).For(controller.For())
		for _, resource := range controller.Owns() {
			builder = builder.Owns(resource)
		}
		Expect(builder.Complete(&GenericController{Controller: controller, Client: m.GetClient()})).
			To(Succeed(), "Failed to register controller to manager for %s", controller.For())

		Expect(controllerruntime.NewWebhookManagedBy(m).For(controller.For()).Complete()).
			To(Succeed(), "Failed to register controller to manager for %s", controller.For())
	}
}

func podSchedulingIndex(object runtime.Object) []string {
	pod, ok := object.(*v1.Pod)
	if !ok {
		return nil
	}
	return []string{pod.Spec.NodeName}
}
