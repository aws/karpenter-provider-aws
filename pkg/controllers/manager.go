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
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis"

	"golang.org/x/time/rate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apis.AddToScheme(scheme)
}

type GenericControllerManager struct {
	manager.Manager
}

// NewManagerOrDie instantiates a controller manager or panics
func NewManagerOrDie(config *rest.Config, options controllerruntime.Options) Manager {
	options.Scheme = scheme
	manager, err := controllerruntime.NewManager(config, options)
	if err != nil {
		panic(fmt.Sprintf("Failed to create controller manager, %s", err.Error()))
	}
	if err := manager.GetFieldIndexer().IndexField(context.Background(), &v1.Pod{}, "spec.nodeName", podSchedulingIndex); err != nil {
		panic(fmt.Sprintf("Failed to setup pod indexer, %s", err.Error()))
	}
	return &GenericControllerManager{Manager: manager}
}

// RegisterControllers registers a set of controllers to the controller manager
func (m *GenericControllerManager) RegisterControllers(controllers ...Controller) Manager {
	for _, c := range controllers {
		controlledObject := c.For()
		builder := controllerruntime.NewControllerManagedBy(m).
			For(controlledObject).
			Watches(c.Watches(context.Background())).
			WithOptions(controller.Options{
				RateLimiter: workqueue.NewMaxOfRateLimiter(
					workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 10*time.Second),
					// 10 qps, 100 bucket size
					&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
				),
			})
		if namedController, ok := c.(NamedController); ok {
			builder.Named(namedController.Name())
		}
		for _, resource := range c.Owns() {
			builder = builder.Owns(resource)
		}
		if err := builder.Complete(&GenericController{Controller: c, Client: m.GetClient()}); err != nil {
			panic(fmt.Sprintf("Failed to register controller to manager for %s", controlledObject))
		}
	}
	if err := m.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		panic(fmt.Sprintf("Failed to add readiness probe, %s", err.Error()))
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
