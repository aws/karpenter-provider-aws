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

package webhooks

import (
	"context"
	"flag"
	"fmt"
	"runtime/debug"

	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	knativeinjection "knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/configmaps"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/env"
	"github.com/aws/karpenter/pkg/utils/injection"
)

type WebhookOpts struct {
	KarpenterService string
	WebhookPort      int
	MemoryLimit      int64
}

var (
	component = "webhook"
	opts      = WebhookOpts{}
)

func init() {
	flag.StringVar(&opts.KarpenterService, "karpenter-service", env.WithDefaultString("KARPENTER_SERVICE", ""), "The Karpenter Service name for the dynamic webhook certificate")
	flag.IntVar(&opts.WebhookPort, "port", env.WithDefaultInt("PORT", 8443), "The port the webhook endpoint binds to for validation and mutation of resources")
	flag.Int64Var(&opts.MemoryLimit, "memory-limit", env.WithDefaultInt64("MEMORY_LIMIT", -1), "Memory limit on the container running the webhook. The GC soft memory limit is set to 90% of this value.")
}

func Initialize(injectCloudProvider func(context.Context, cloudprovider.Options) cloudprovider.CloudProvider) {
	config := knativeinjection.ParseAndGetRESTConfigOrDie()

	// Set up logger and watch for changes to log level
	clientSet := kubernetes.NewForConfigOrDie(config)
	cmw := informer.NewInformedWatcher(clientSet, system.Namespace())
	ctx := injection.LoggingContextOrDie(component, config, cmw)
	ctx = knativeinjection.WithNamespaceScope(ctx, system.Namespace())

	ctx = webhook.WithOptions(ctx, webhook.Options{
		Port:        opts.WebhookPort,
		ServiceName: opts.KarpenterService,
		SecretName:  fmt.Sprintf("%s-cert", opts.KarpenterService),
	})
	// TODO: This can be removed if we eventually decide that we need leader election. Having leader election has resulted in the webhook
	// having issues described in https://github.com/aws/karpenter/issues/2562 so these issues need to be resolved if this line is removed
	ctx = sharedmain.WithHADisabled(ctx) // Disable leader election for webhook

	// Register the cloud provider to attach vendor specific validation logic.
	// TODO(https://github.com/aws/karpenter/issues/2052)
	injectCloudProvider(ctx, cloudprovider.Options{
		ClientSet:   clientSet,
		WebhookOnly: true,
	})

	if opts.MemoryLimit > 0 {
		newLimit := int64(float64(opts.MemoryLimit) * 0.9)
		logging.FromContext(ctx).Infof("Setting GC memory limit to %d, container limit = %d", newLimit, opts.MemoryLimit)
		debug.SetMemoryLimit(newLimit)
	}

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
