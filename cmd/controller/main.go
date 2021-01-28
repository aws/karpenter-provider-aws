package main

import (
	"flag"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers"

	"github.com/awslabs/karpenter/pkg/autoscaler"
	horizontalautoscalerv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1"
	metricsproducerv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/metricsproducer/v1alpha1"
	provisionerv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/provisioner/v1alpha1"
	"github.com/awslabs/karpenter/pkg/controllers/provisioner/v1alpha1/allocation"
	scalablenodegroupv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/scalablenodegroup/v1alpha1"
	metricsclients "github.com/awslabs/karpenter/pkg/metrics/clients"
	"github.com/awslabs/karpenter/pkg/metrics/producers"

	"github.com/awslabs/karpenter/pkg/utils/log"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme  = runtime.NewScheme()
	options = Options{}
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apis.AddToScheme(scheme)
}

// Options for running this binary
type Options struct {
	EnableVerboseLogging bool
	MetricsPort          int
	WebhookPort          int
	PrometheusURI        string
}

func main() {
	flag.BoolVar(&options.EnableVerboseLogging, "verbose", false, "Enable verbose logging.")
	flag.StringVar(&options.PrometheusURI, "prometheus-uri", "http://prometheus-operated:9090", "The Prometheus Metrics Server URI for retrieving metrics")
	flag.IntVar(&options.WebhookPort, "webhook-port", 9443, "The port the webhook endpoint binds to for validation and mutation of resources.")
	flag.IntVar(&options.MetricsPort, "metrics-port", 8080, "The port the metric endpoint binds to for operating metrics about the controller itself.")
	flag.Parse()

	log.Setup(controllerruntimezap.UseDevMode(options.EnableVerboseLogging))

	manager := controllers.NewManagerOrDie(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		LeaderElection:     true,
		LeaderElectionID:   "karpenter-leader-election",
		Scheme:             scheme,
		MetricsBindAddress: fmt.Sprintf(":%d", options.MetricsPort),
		Port:               options.WebhookPort,
	})

	cloudProviderFactory := registry.NewFactory(cloudprovider.Options{Client: manager.GetClient(), Config: manager.GetConfig()})
	metricsProducerFactory := &producers.Factory{Client: manager.GetClient(), CloudProviderFactory: cloudProviderFactory}
	metricsClientFactory := metricsclients.NewFactoryOrDie(options.PrometheusURI)
	autoscalerFactory := autoscaler.NewFactoryOrDie(metricsClientFactory, manager.GetRESTMapper(), manager.GetConfig())

	if err := manager.Register(
		&horizontalautoscalerv1alpha1.Controller{AutoscalerFactory: autoscalerFactory},
		&scalablenodegroupv1alpha1.Controller{CloudProvider: cloudProviderFactory},
		&metricsproducerv1alpha1.Controller{ProducerFactory: metricsProducerFactory},
		&provisionerv1alpha1.Controller{
			Client: manager.GetClient(),
			Allocator: &allocation.GreedyAllocator{
				Capacity: cloudProviderFactory.Capacity(),
			},
			ProcessedPods: make(map[string]bool),
		},
	).Start(controllerruntime.SetupSignalHandler()); err != nil {
		zap.S().Panicf("Unable to start manager, %w", err)
	}
}
