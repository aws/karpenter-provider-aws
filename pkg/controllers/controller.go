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
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// Controller is implemented by all resource controllers
type Controller interface {
	Reconcile(req controllerruntime.Request) (controllerruntime.Result, error)
	For() runtime.Object
	Owns() []runtime.Object
}

// RegisterController registers the provided Controller as a controller in the controller Manager.
func RegisterController(manager controllerruntime.Manager, controller Controller) error {
	var builder = controllerruntime.NewControllerManagedBy(manager).For(controller.For())
	for _, resource := range controller.Owns() {
		builder = builder.Owns(resource)
	}
	return errors.Wrapf(builder.Complete(controller), "registering controller to manager for resource %v", controller.For())
}

// RegisterWebhook registers the provided Controller as a webhook in the controller Manager.
func RegisterWebhook(manager controllerruntime.Manager, controller Controller) error {
	return errors.Wrapf(
		controllerruntime.NewWebhookManagedBy(manager).For(controller.For()).Complete(),
		"registering webhook to manager for resource %v", controller.For())
}

func generateMutatePath(gvk schema.GroupVersionKind) string {
	return "/mutate-" + strings.Replace(gvk.Group, ".", "-", -1) + "-" +
		gvk.Version + "-" + strings.ToLower(gvk.Kind)
}
