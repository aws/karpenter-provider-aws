package main

import (
	"flag"
	"fmt"

	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup"
	"github.com/ellistarn/karpenter/pkg/controllers"

	"github.com/ellistarn/karpenter/pkg/autoscaler"
	horizontalautoscalerv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1"
	metricsproducerv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/metricsproducer/v1alpha1"
	scalablenodegroupv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/scalablenodegroup/v1alpha1"
	metricsclients "github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/ellistarn/karpenter/pkg/metrics/producers"

	"github.com/ellistarn/karpenter/pkg/utils/log"
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

	metricsProducerFactory := &producers.Factory{Client: manager.GetClient()}
	metricsClientFactory := metricsclients.NewFactoryOrDie(options.PrometheusURI)
	autoscalerFactory := autoscaler.NewFactoryOrDie(metricsClientFactory, manager.GetRESTMapper(), manager.GetConfig())

	horizontalAutoscalerController := &horizontalautoscalerv1alpha1.Controller{AutoscalerFactory: autoscalerFactory}
	scalableNodeGroupController := &scalablenodegroupv1alpha1.Controller{NodeGroupFactory: &nodegroup.Factory{}}
	metricsProducerController := &metricsproducerv1alpha1.Controller{ProducerFactory: metricsProducerFactory}

	manager.Register(
		horizontalAutoscalerController,
		scalableNodeGroupController,
		metricsProducerController,
	)
	if err := manager.Start(controllerruntime.SetupSignalHandler()); err != nil {
		zap.S().Fatalf("Unable to start manager, %w", err)
	}
}
