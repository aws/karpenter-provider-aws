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

package operator

import (
	"context"
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"knative.dev/pkg/configmap/informer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/utils/options"
)

// Options exposes shared components that are initialized by the startup.Initialize() call
type Options struct {
	Ctx        context.Context
	Recorder   events.Recorder
	Config     *rest.Config
	KubeClient client.Client
	Clientset  *kubernetes.Clientset
	Clock      clock.Clock
	Options    *options.Options
	Cmw        *informer.InformedWatcher
	StartAsync <-chan struct{}
}

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(context.Context, reconcile.Request) (reconcile.Result, error)
	// Register will register the controller with the manager
	Register(context.Context, manager.Manager) error
}

// HealthCheck is an interface for a controller that exposes a LivenessProbe
type HealthCheck interface {
	LivenessProbe(req *http.Request) error
}
