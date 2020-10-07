package main

import (
	"flag"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup"
	"github.com/ellistarn/karpenter/pkg/controllers"

	"github.com/ellistarn/karpenter/pkg/autoscaler"
	horizontalautoscalerv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1"
	metricsproducerv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/metricsproducer/v1alpha1"
	scalablenodegroupv1alpha1 "github.com/ellistarn/karpenter/pkg/controllers/scalablenodegroup/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	metricsclients "github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/ellistarn/karpenter/pkg/metrics/producers"
	metricsproducers "github.com/ellistarn/karpenter/pkg/metrics/producers"
	"github.com/prometheus/client_golang/api"

	"github.com/ellistarn/karpenter/pkg/utils/log"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/scale"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	// +kubebuilder:scaffold:imports
)

var (
	scheme       = runtime.NewScheme()
	options      = Options{}
	dependencies = Dependencies{}
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apis.AddToScheme(scheme)
}

// Options for running this binary
type Options struct {
	EnableLeaderElection bool
	EnableWebhook        bool
	EnableController     bool
	EnableVerboseLogging bool
	MetricsAddr          string
	PrometheusURI        string
}

// Dependencies to be injected
type Dependencies struct {
	Manager                manager.Manager
	InformerFactory        informers.SharedInformerFactory
	Controllers            []controllers.Controller
	MetricsProducerFactory metricsproducers.Factory
	MetricsClientFactory   metricsclients.Factory
	AutoscalerFactory      autoscaler.Factory
	NodeGroupFactory       nodegroup.Factory
}

func main() {
	setupFlags()
	log.Setup(controllerruntimezap.UseDevMode(options.EnableVerboseLogging))

	dependencies.Manager = managerOrDie()
	dependencies.InformerFactory = informerFactoryOrDie()
	dependencies.MetricsProducerFactory = metricsProducerFactoryOrDie()
	dependencies.MetricsClientFactory = metricsClientFactoryOrDie()
	dependencies.AutoscalerFactory = autoscalerFactoryOrDie()
	dependencies.Controllers = controllersOrDie()

	if err := dependencies.Manager.Start(controllerruntime.SetupSignalHandler()); err != nil {
		zap.S().Fatalf("Unable to start manager, %v", err)
	}
}

func setupFlags() {
	// Controller
	flag.BoolVar(&options.EnableLeaderElection, "enable-leader-election", true, "Enable leader election.")
	flag.BoolVar(&options.EnableWebhook, "enable-webhook", true, "Enable webhook.")
	flag.BoolVar(&options.EnableController, "enable-controller", true, "Enable controller.")
	flag.BoolVar(&options.EnableVerboseLogging, "verbose", true, "Enable verbose logging.")

	// Metrics
	flag.StringVar(&options.PrometheusURI, "prometheus-uri", "http://kube-prometheus-stack-prometheus.monitoring.svc.cluster.local:9090", "The Prometheus Metrics Server URI for retrieving metrics")
	flag.StringVar(&options.MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to for operating metrics about the controller itself.")
	flag.Parse()
}

func managerOrDie() manager.Manager {
	mgr, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		Scheme:             scheme,
		MetricsBindAddress: options.MetricsAddr,
		Port:               9443,
		LeaderElection:     options.EnableLeaderElection,
		LeaderElectionID:   "karpenter-leader-election",
	})
	if err != nil {
		zap.S().Fatalf("Unable to start controller manager, %v", err)
	}
	return mgr
}

func informerFactoryOrDie() informers.SharedInformerFactory {
	factory := informers.NewSharedInformerFactory(
		kubernetes.NewForConfigOrDie(dependencies.Manager.GetConfig()),
		time.Minute*30,
	)

	if err := dependencies.Manager.Add(manager.RunnableFunc(func(stopChannel <-chan struct{}) error {
		factory.Start(stopChannel)
		<-stopChannel
		return nil
	})); err != nil {
		zap.S().Fatalf("Unable to register informer factory, %v", err)
	}

	return factory
}

func metricsProducerFactoryOrDie() producers.Factory {
	return producers.Factory{
		NodeLister: dependencies.InformerFactory.Core().V1().Nodes().Lister(),
		PodLister:  dependencies.InformerFactory.Core().V1().Pods().Lister(),
	}
}

func metricsClientFactoryOrDie() clients.Factory {
	client, err := api.NewClient(api.Config{Address: options.PrometheusURI})
	if err != nil {
		zap.S().Fatalf("Unable to create prometheus client, %v", err)
	}
	return clients.Factory{
		PrometheusClient: prometheusv1.NewAPI(client),
	}
}

func autoscalerFactoryOrDie() autoscaler.Factory {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(dependencies.Manager.GetConfig())
	if err != nil {
		zap.S().Fatalf("Unable to create discovery client, %v", err)
	}
	scale, err := scale.NewForConfig(
		dependencies.Manager.GetConfig(),
		dependencies.Manager.GetRESTMapper(),
		dynamic.LegacyAPIPathResolverFunc,
		scale.NewDiscoveryScaleKindResolver(discoveryClient),
	)
	if err != nil {
		zap.S().Fatalf("Unable to create scale client, %v", err)
	}
	return autoscaler.Factory{
		MetricsClientFactory: dependencies.MetricsClientFactory,
		Mapper:               dependencies.Manager.GetRESTMapper(),
		ScaleNamespacer:      scale,
	}
}

func controllersOrDie() []controllers.Controller {
	cs := []controllers.Controller{
		&horizontalautoscalerv1alpha1.Controller{
			AutoscalerFactory: dependencies.AutoscalerFactory,
		},
		&scalablenodegroupv1alpha1.Controller{
			NodeGroupFactory: dependencies.NodeGroupFactory,
		},
		&metricsproducerv1alpha1.Controller{
			ProducerFactory: dependencies.MetricsProducerFactory,
		},
	}
	for _, c := range cs {
		if err := controllers.Register(dependencies.Manager, c); err != nil {
			zap.S().Fatalf("Failed to register controller for resource %v: %v", c.For(), err)
		}
	}
	return cs
}
