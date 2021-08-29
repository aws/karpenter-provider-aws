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

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
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
	Port int
}

func main() {
	flag.IntVar(&options.Port, "port", 8443, "The port the webhook endpoint binds to for validation and mutation of resources")
	flag.Parse()

	config := injection.ParseAndGetRESTConfigOrDie()
	ctx := webhook.WithOptions(injection.WithNamespaceScope(signals.NewContext(), system.Namespace()), webhook.Options{
		Port:        options.Port,
		ServiceName: "karpenter-webhook",
		SecretName:  "karpenter-webhook-cert",
	})

	// Register the cloud provider to attach vendor specific validation logic.
	registry.NewCloudProvider(ctx, cloudprovider.Options{ClientSet: kubernetes.NewForConfigOrDie(config)})

	// Controllers and webhook
	sharedmain.MainWithConfig(ctx, "webhook", config,
		certificates.NewController,
		newCRDDefaultingWebhook,
		newCRDValidationWebhook,
		newConfigValidationController,
	)
}

func newCRDDefaultingWebhook(ctx context.Context, w configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,
		"defaulting.webhook.provisioners.karpenter.sh",
		"/default-resource",
		apis.Resources,
		InjectContext,
		true,
	)
}

func newCRDValidationWebhook(ctx context.Context, w configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		"validation.webhook.provisioners.karpenter.sh",
		"/validate-resource",
		apis.Resources,
		InjectContext,
		true,
	)
}

func newConfigValidationController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return configmaps.NewAdmissionController(ctx,
		"validation.webhook.config.karpenter.sh",
		"/config-validation",
		configmap.Constructors{
			logging.ConfigMapName(): logging.NewConfigFromConfigMap,
		},
	)
}

func InjectContext(ctx context.Context) context.Context {
	return ctx
}
