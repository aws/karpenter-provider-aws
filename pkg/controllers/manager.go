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

package controllers

import (
	"context"
	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/utils/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	log.PanicIfError(clientgoscheme.AddToScheme(scheme), "adding clientgo to scheme")
	log.PanicIfError(apis.AddToScheme(scheme), "adding apis to scheme")
}

type Manager interface {
	manager.Manager
	Register(controllers ...Controller) Manager
}

type GenericControllerManager struct {
	manager.Manager
}

// NewManager instantiates a controller manager or panics
func NewManagerOrDie(config *rest.Config, options controllerruntime.Options) Manager {
	options.Scheme = scheme
	manager, err := controllerruntime.NewManager(config, options)
	log.PanicIfError(err, "Failed to create controller manager")
	log.PanicIfError(manager.GetFieldIndexer().
		IndexField(context.Background(), &v1.Pod{}, "spec.nodeName", podSchedulingIndex), "Failed to setup pod indexer")
	return &GenericControllerManager{Manager: manager}
}

func (m *GenericControllerManager) Register(controllers ...Controller) Manager {
	for _, controller := range controllers {
		controlledObject := controller.For()
		var builder = controllerruntime.NewControllerManagedBy(m).For(controlledObject)
		if namedController, ok := controller.(NamedController); ok {
			builder.Named(namedController.Name())
		}
		for _, resource := range controller.Owns() {
			builder = builder.Owns(resource)
		}
		log.PanicIfError(builder.Complete(&GenericController{Controller: controller, Client: m.GetClient()}),
			"Failed to register controller to manager for %s", controlledObject)
		log.PanicIfError(controllerruntime.NewWebhookManagedBy(m).For(controlledObject).Complete(),
			"Failed to register controller to manager for %s", controlledObject)
	}
	return m
}

func podSchedulingIndex(object client.Object) []string {
	pod, ok := object.(*v1.Pod)
	if !ok {
		return nil
	}
	return []string{pod.Spec.NodeName}
}
