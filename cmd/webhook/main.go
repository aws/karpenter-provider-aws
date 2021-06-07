package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

var (
	options = Options{}
)

type Options struct {
	Port                  int
	ServiceName           string
	CertificateSecretName string
}

func main() {
	flag.IntVar(&options.Port, "port", 8443, "The port the webhook endpoint binds to for validation and mutation of resources")
	flag.StringVar(&options.ServiceName, "service-name", "karpenter-webhook", "The name of the webhook's service")
	flag.StringVar(&options.CertificateSecretName, "certificate-secret-name", "karpenter-webhook-cert", "The name of the webhook's secret containing certificates")
	flag.Parse()

	// Register the cloud provider to attach vendor specific validation logic.
	registry.New(cloudprovider.Options{ClientSet: kubernetes.NewForConfigOrDie(sharedmain.ParseAndGetConfigOrDie())})

	sharedmain.MainWithContext(
		webhook.WithOptions(injection.WithNamespaceScope(signals.NewContext(), system.Namespace()), webhook.Options{
			ServiceName: options.ServiceName,
			Port:        options.Port,
			SecretName:  options.CertificateSecretName,
		}),
		options.ServiceName,
		certificates.NewController,
		func(ctx context.Context, w configmap.Watcher) *controller.Impl {
			return defaulting.NewAdmissionController(ctx,
				fmt.Sprintf("defaulting.%s", v1alpha1.SchemeGroupVersion.Group),
				"/default",
				apis.Resources,
				InjectContext,
				true,
			)
		},
		func(ctx context.Context, w configmap.Watcher) *controller.Impl {
			return validation.NewAdmissionController(ctx,
				fmt.Sprintf("validation.%s", v1alpha1.SchemeGroupVersion.Group),
				"/validate",
				apis.Resources,
				InjectContext,
				true,
			)
		},
	)
}

func InjectContext(ctx context.Context) context.Context { return ctx }
