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

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	knativeinjection "knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	"github.com/aws/karpenter/pkg/apis"
	awsapis "github.com/aws/karpenter/pkg/cloudprovider/aws/apis"
	"github.com/aws/karpenter/pkg/utils/env"
)

type WebhookOpts struct {
	KarpenterService string
	WebhookPort      int
}

var opts = WebhookOpts{}

func init() {
	flag.StringVar(&opts.KarpenterService, "karpenter-service", env.WithDefaultString("KARPENTER_SERVICE", ""), "The Karpenter Service name for the dynamic webhook certificate")
	flag.IntVar(&opts.WebhookPort, "port", env.WithDefaultInt("PORT", 8443), "The port the webhook endpoint binds to for validation and mutation of resources")
}

func main() {
	config := knativeinjection.ParseAndGetRESTConfigOrDie()
	ctx := webhook.WithOptions(knativeinjection.WithNamespaceScope(signals.NewContext(), system.Namespace()), webhook.Options{
		Port:        opts.WebhookPort,
		ServiceName: opts.KarpenterService,
		SecretName:  fmt.Sprintf("%s-cert", opts.KarpenterService),
	})

	// Controllers and webhook
	sharedmain.MainWithConfig(ctx, "webhook", config,
		certificates.NewController,
		newCRDDefaultingWebhook,
		newCRDValidationWebhook,
		newConfigValidationController,
		// AWS Specific Webhooks
		newAWSDefaultingWebhook,
		newAWSValidationWebhook,
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

func newAWSDefaultingWebhook(ctx context.Context, w configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,
		"defaulting.webhook.karpenter.k8s.aws",
		"/aws/default-resource",
		awsapis.Resources,
		InjectContext,
		true,
	)
}

func newAWSValidationWebhook(ctx context.Context, w configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		"validation.webhook.karpenter.k8s.aws",
		"/aws/validate-resource",
		awsapis.Resources,
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
