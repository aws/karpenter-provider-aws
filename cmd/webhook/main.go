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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var (
	options = Options{}
)

type Options struct {
	Port            int
	HealthProbePort int
}

func main() {
	flag.IntVar(&options.Port, "port", 8443, "The port the webhook endpoint binds to for validation and mutation of resources")
	flag.IntVar(&options.HealthProbePort, "health-probe-port", 8081, "The port the health probe endpoint binds to for reporting controller health")
	flag.Parse()

	config := sharedmain.ParseAndGetConfigOrDie()

	// Register the cloud provider to attach vendor specific validation logic.
	registry.NewCloudProvider(cloudprovider.Options{ClientSet: kubernetes.NewForConfigOrDie(config)})

	// Liveness handler
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", options.HealthProbePort), mux))
	}()

	// Controllers and webhook
	sharedmain.MainWithConfig(
		webhook.WithOptions(injection.WithNamespaceScope(signals.NewContext(), system.Namespace()), webhook.Options{
			Port:        options.Port,
			ServiceName: "karpenter-webhook",
			SecretName:  "karpenter-webhook-cert",
		}),
		"karpenter.webhooks",
		config,
		certificates.NewController,
		NewCRDDefaultingWebhook,
		NewCRDValidationWebhook,
		NewConfigmapValidationWebhook,
	)
}

func NewCRDDefaultingWebhook(ctx context.Context, w configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,
		"defaulting.webhook.provisioners.karpenter.sh",
		"/default-resource",
		apis.Resources,
		InjectContext,
		true,
	)
}

func NewCRDValidationWebhook(ctx context.Context, w configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		"validation.webhook.provisioners.karpenter.sh",
		"/validate-resource",
		apis.Resources,
		InjectContext,
		true,
	)
}

func NewConfigmapValidationWebhook(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,
		"validation.webhook.configmaps.karpenter.sh",
		"/validate-config",
		configmap.Constructors{
			logging.ConfigMapName():        logging.NewConfigFromConfigMap,
			leaderelection.ConfigMapName(): leaderelection.NewConfigFromConfigMap,
		},
	)
}

func InjectContext(ctx context.Context) context.Context { return ctx }
