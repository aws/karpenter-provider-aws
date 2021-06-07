package main

import (
	"flag"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation"
	termination "github.com/awslabs/karpenter/pkg/controllers/terminating/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/log"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	HealthProbePort      int
}

func main() {
	flag.BoolVar(&options.EnableVerboseLogging, "verbose", false, "Enable verbose logging")
	flag.IntVar(&options.MetricsPort, "metrics-port", 8080, "The port the metric endpoint binds to for operating metrics about the controller itself")
	flag.IntVar(&options.HealthProbePort, "health-probe-port", 8081, "The port the health probe endpoint binds to for reporting controller health")
	flag.Parse()

	log.Setup(
		controllerruntimezap.UseDevMode(options.EnableVerboseLogging),
		controllerruntimezap.ConsoleEncoder(),
		controllerruntimezap.StacktraceLevel(zapcore.DPanicLevel),
	)
	manager := controllers.NewManagerOrDie(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		LeaderElection:         true,
		LeaderElectionID:       "karpenter-leader-election",
		Scheme:                 scheme,
		MetricsBindAddress:     fmt.Sprintf(":%d", options.MetricsPort),
		HealthProbeBindAddress: fmt.Sprintf(":%d", options.HealthProbePort),
	})

	clientSet := kubernetes.NewForConfigOrDie(manager.GetConfig())
	cloudProvider := registry.New(cloudprovider.Options{ClientSet: clientSet})

	err := manager.RegisterControllers(
		allocation.NewController(manager.GetClient(), clientSet.CoreV1(), cloudProvider),
		reallocation.NewController(manager.GetClient(), clientSet.CoreV1(), cloudProvider),
		termination.NewController(manager.GetClient(), clientSet.CoreV1(), cloudProvider),
	).Start(controllerruntime.SetupSignalHandler())
	log.PanicIfError(err, "Unable to start manager")
}
