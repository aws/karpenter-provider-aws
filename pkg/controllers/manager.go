package controllers

import (
	"context"

	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/utils/log"
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
	log.PanicIfError(clientgoscheme.AddToScheme(scheme), "adding clientgo to scheme")
	log.PanicIfError(apis.AddToScheme(scheme), "adding apis to scheme")
}

type Manager struct {
	manager.Manager
}

// NewManager instantiates a controller manager or panics
func NewManagerOrDie(config *rest.Config, options controllerruntime.Options) Manager {
	options.Scheme = scheme
	manager, err := controllerruntime.NewManager(config, options)
	log.PanicIfError(err, "Failed to create controller manager")
	log.PanicIfError(manager.GetFieldIndexer().
		IndexField(context.Background(), &v1.Pod{}, "spec.nodeName", podSchedulingIndex),
		"Failed to setup pod indexer")
	return Manager{Manager: manager}
}

func (m *Manager) Register(controllers ...Controller) {
	for _, controller := range controllers {
		var builder = controllerruntime.NewControllerManagedBy(m).For(controller.For())
		for _, resource := range controller.Owns() {
			builder = builder.Owns(resource)
		}
		log.PanicIfError(builder.Complete(&GenericController{Controller: controller, Client: m.GetClient()}),
			"Failed to register controller to manager for %s", controller.For())
		log.PanicIfError(controllerruntime.NewWebhookManagedBy(m).For(controller.For()).Complete(),
			"Failed to register controller to manager for %s", controller.For())
	}
}

func podSchedulingIndex(object runtime.Object) []string {
	pod, ok := object.(*v1.Pod)
	if !ok {
		return nil
	}
	return []string{pod.Spec.NodeName}
}
