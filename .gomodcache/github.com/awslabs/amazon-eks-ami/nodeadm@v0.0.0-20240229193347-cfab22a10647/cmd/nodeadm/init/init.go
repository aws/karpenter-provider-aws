package init

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
	"k8s.io/utils/strings/slices"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/api"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/aws/ecr"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/cli"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/configprovider"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/containerd"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/daemon"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/kubelet"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/system"
)

const (
	configPhase = "config"
	runPhase    = "run"
)

func NewInitCommand() cli.Command {
	init := initCmd{}
	init.cmd = flaggy.NewSubcommand("init")
	init.cmd.StringSlice(&init.daemons, "d", "daemon", "specify one or more of `containerd` and `kubelet`. This is intended for testing and should not be used in a production environment.")
	init.cmd.StringSlice(&init.skipPhases, "s", "skip", "phases of the bootstrap you want to skip")
	init.cmd.Description = "Initialize this instance as a node in an EKS cluster"
	return &init
}

type initCmd struct {
	cmd        *flaggy.Subcommand
	skipPhases []string
	daemons    []string
}

func (c *initCmd) Flaggy() *flaggy.Subcommand {
	return c.cmd
}

func (c *initCmd) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	log.Info("Checking user is root..")
	root, err := cli.IsRunningAsRoot()
	if err != nil {
		return err
	} else if !root {
		return cli.ErrMustRunAsRoot
	}

	log.Info("Loading configuration..", zap.String("configSource", opts.ConfigSource))
	provider, err := configprovider.BuildConfigProvider(opts.ConfigSource)
	if err != nil {
		return err
	}
	nodeConfig, err := provider.Provide()
	if err != nil {
		return err
	}
	log.Info("Loaded configuration", zap.Reflect("config", nodeConfig))

	log.Info("Enriching configuration..")
	if err := enrichConfig(log, nodeConfig); err != nil {
		return err
	}

	zap.L().Info("Validating configuration..")
	if err := api.ValidateNodeConfig(nodeConfig); err != nil {
		return err
	}

	log.Info("Creating daemon manager..")
	daemonManager, err := daemon.NewDaemonManager()
	if err != nil {
		return err
	}
	defer daemonManager.Close()

	aspects := []system.SystemAspect{
		system.NewLocalDiskAspect(),
	}

	daemons := []daemon.Daemon{
		containerd.NewContainerdDaemon(daemonManager),
		kubelet.NewKubeletDaemon(daemonManager),
	}

	if !slices.Contains(c.skipPhases, configPhase) {
		log.Info("Configuring daemons...")
		for _, daemon := range daemons {
			if len(c.daemons) > 0 && !slices.Contains(c.daemons, daemon.Name()) {
				continue
			}
			nameField := zap.String("name", daemon.Name())

			log.Info("Configuring daemon...", nameField)
			if err := daemon.Configure(nodeConfig); err != nil {
				return err
			}
			log.Info("Configured daemon", nameField)
		}
	}

	if !slices.Contains(c.skipPhases, runPhase) {
		log.Info("Setting up system aspects...")
		for _, aspect := range aspects {
			nameField := zap.String("name", aspect.Name())
			log.Info("Setting up system aspect..", nameField)
			if err := aspect.Setup(nodeConfig); err != nil {
				return err
			}
			log.Info("Set up system aspect", nameField)
		}
		for _, daemon := range daemons {
			if len(c.daemons) > 0 && !slices.Contains(c.daemons, daemon.Name()) {
				continue
			}

			nameField := zap.String("name", daemon.Name())

			log.Info("Ensuring daemon is running..", nameField)
			if err := daemon.EnsureRunning(); err != nil {
				return err
			}
			log.Info("Daemon is running", nameField)

			log.Info("Running post-launch tasks..", nameField)
			if err := daemon.PostLaunch(nodeConfig); err != nil {
				return err
			}
			log.Info("Finished post-launch tasks", nameField)
		}
	}

	return nil
}

// Various initializations and verifications of the NodeConfig and
// perform in-place updates when allowed by the user
func enrichConfig(log *zap.Logger, cfg *api.NodeConfig) error {
	log.Info("Fetching instance details..")
	instanceDetails, err := api.GetIMDSInstanceDetails(context.TODO(), imds.New(imds.Options{}))
	if err != nil {
		return err
	}
	cfg.Status.Instance = *instanceDetails
	log.Info("Instance details populated", zap.Reflect("details", instanceDetails))
	log.Info("Fetching default options...")
	eksRegistry, err := ecr.GetEKSRegistry(instanceDetails.Region)
	if err != nil {
		return err
	}
	cfg.Status.Defaults = api.DefaultOptions{
		SandboxImage: eksRegistry.GetSandboxImage(),
	}
	log.Info("Default options populated", zap.Reflect("defaults", cfg.Status.Defaults))
	return nil
}

// Discovers all cluster details using a describe call to the eks endpoint and
// updates the value of the config's `ClusterDetails` in-place
func populateClusterDetails(eksClient *eks.EKS, clusterName string, cfg *api.NodeConfig) error {
	if err := eksClient.WaitUntilClusterActive(&eks.DescribeClusterInput{Name: &clusterName}); err != nil {
		return err
	}
	describeResponse, err := eksClient.DescribeCluster(&eks.DescribeClusterInput{Name: &clusterName})
	if err != nil {
		return err
	}

	ipFamily := *describeResponse.Cluster.KubernetesNetworkConfig.IpFamily

	var cidr string
	if ipFamily == eks.IpFamilyIpv4 {
		cidr = *describeResponse.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr
	} else if ipFamily == eks.IpFamilyIpv6 {
		cidr = *describeResponse.Cluster.KubernetesNetworkConfig.ServiceIpv6Cidr
	} else {
		return fmt.Errorf("bad ipFamily: %s", ipFamily)
	}

	isOutpost := false
	clusterId := cfg.Spec.Cluster.ID
	// detect whether the cluster is an aws outpost cluster depending on whether
	// the response contains the outpost ID
	if outpostId := describeResponse.Cluster.Id; outpostId != nil {
		clusterId = *outpostId
		isOutpost = true
	}

	enableOutpost := isOutpost
	// respect the user override for enabling the outpost
	if enabled := cfg.Spec.Cluster.EnableOutpost; enabled != nil {
		enableOutpost = *enabled
	}

	caCert, err := base64.StdEncoding.DecodeString(*describeResponse.Cluster.CertificateAuthority.Data)
	if err != nil {
		return err
	}

	cfg.Spec.Cluster.Name = *describeResponse.Cluster.Name
	cfg.Spec.Cluster.APIServerEndpoint = *describeResponse.Cluster.Endpoint
	cfg.Spec.Cluster.CertificateAuthority = caCert
	cfg.Spec.Cluster.CIDR = cidr
	cfg.Spec.Cluster.EnableOutpost = &enableOutpost
	cfg.Spec.Cluster.ID = clusterId

	return nil
}
