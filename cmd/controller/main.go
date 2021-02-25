package main

import (
	"flag"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/autoscaler"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers"
	horizontalautoscalerv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1"
	metricsproducerv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/metricsproducer/v1alpha1"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation"
	scalablenodegroupv1alpha1 "github.com/awslabs/karpenter/pkg/controllers/scalablenodegroup/v1alpha1"
	metricsclients "github.com/awslabs/karpenter/pkg/metrics/clients"
	"github.com/awslabs/karpenter/pkg/metrics/producers"

	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/allocation"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/kubernetes"
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

	log.Setup(controllerruntimezap.UseDevMode(options.EnableVerboseLogging), controllerruntimezap.ConsoleEncoder())
	manager := controllers.NewManagerOrDie(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		LeaderElection:     true,
		LeaderElectionID:   "karpenter-leader-election",
		Scheme:             scheme,
		MetricsBindAddress: fmt.Sprintf(":%d", options.MetricsPort),
		Port:               options.WebhookPort,
	})

	clientSet := kubernetes.NewForConfigOrDie(manager.GetConfig())
	cloudProviderFactory := registry.NewFactory(cloudprovider.Options{Client: manager.GetClient(), ClientSet: clientSet})
	metricsProducerFactory := &producers.Factory{Client: manager.GetClient(), CloudProviderFactory: cloudProviderFactory}
	metricsClientFactory := metricsclients.NewFactoryOrDie(options.PrometheusURI)
	autoscalerFactory := autoscaler.NewFactoryOrDie(metricsClientFactory, manager.GetRESTMapper(), manager.GetConfig())
	err := manager.Register(
		&horizontalautoscalerv1alpha1.Controller{AutoscalerFactory: autoscalerFactory},
		&scalablenodegroupv1alpha1.Controller{CloudProvider: cloudProviderFactory},
		&metricsproducerv1alpha1.Controller{ProducerFactory: metricsProducerFactory},
		allocation.NewController(manager.GetClient(), clientSet.CoreV1(), cloudProviderFactory),
		reallocation.NewController(manager.GetClient(), cloudProviderFactory),
	).Start(controllerruntime.SetupSignalHandler())
	log.PanicIfError(err, "Unable to start manager")
}
