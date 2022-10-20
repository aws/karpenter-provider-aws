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

package options

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"

	"go.uber.org/multierr"

	"github.com/aws/karpenter-core/pkg/utils/env"
)

type AWSNodeNameConvention string

const (
	IPName       AWSNodeNameConvention = "ip-name"
	ResourceName AWSNodeNameConvention = "resource-name"
)

// Options for running this binary
type Options struct {
	*flag.FlagSet
	// Vendor Neutral
	MetricsPort          int
	HealthProbePort      int
	KubeClientQPS        int
	KubeClientBurst      int
	EnableProfiling      bool
	EnableLeaderElection bool
	MemoryLimit          int64
	// AWS Specific
	ClusterName               string
	ClusterEndpoint           string
	VMMemoryOverhead          float64
	AWSNodeNameConvention     string
	AWSENILimitedPodDensity   bool
	AWSDefaultInstanceProfile string
	AWSEnablePodENI           bool
	AWSIsolatedVPC            bool
}

// New creates an Options struct and registers CLI flags and environment variables to fill-in the Options struct fields
func New() *Options {
	opts := &Options{}
	f := flag.NewFlagSet("karpenter", flag.ContinueOnError)
	opts.FlagSet = f

	// Vendor Neutral
	f.IntVar(&opts.MetricsPort, "metrics-port", env.WithDefaultInt("METRICS_PORT", 8080), "The port the metric endpoint binds to for operating metrics about the controller itself")
	f.IntVar(&opts.HealthProbePort, "health-probe-port", env.WithDefaultInt("HEALTH_PROBE_PORT", 8081), "The port the health probe endpoint binds to for reporting controller health")
	f.IntVar(&opts.KubeClientQPS, "kube-client-qps", env.WithDefaultInt("KUBE_CLIENT_QPS", 200), "The smoothed rate of qps to kube-apiserver")
	f.IntVar(&opts.KubeClientBurst, "kube-client-burst", env.WithDefaultInt("KUBE_CLIENT_BURST", 300), "The maximum allowed burst of queries to the kube-apiserver")
	f.BoolVar(&opts.EnableProfiling, "enable-profiling", env.WithDefaultBool("ENABLE_PROFILING", false), "Enable the profiling on the metric endpoint")
	f.BoolVar(&opts.EnableLeaderElection, "leader-elect", env.WithDefaultBool("LEADER_ELECT", true), "Start leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	f.Int64Var(&opts.MemoryLimit, "memory-limit", env.WithDefaultInt64("MEMORY_LIMIT", -1), "Memory limit on the container running the controller. The GC soft memory limit is set to 90% of this value.")

	// AWS Specific
	f.StringVar(&opts.ClusterName, "cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "The kubernetes cluster name for resource discovery")
	f.StringVar(&opts.ClusterEndpoint, "cluster-endpoint", env.WithDefaultString("CLUSTER_ENDPOINT", ""), "The external kubernetes cluster endpoint for new nodes to connect with")
	f.Float64Var(&opts.VMMemoryOverhead, "vm-memory-overhead", env.WithDefaultFloat64("VM_MEMORY_OVERHEAD", 0.075), "The VM memory overhead as a percent that will be subtracted from the total memory for all instance types")
	f.StringVar(&opts.AWSNodeNameConvention, "aws-node-name-convention", env.WithDefaultString("AWS_NODE_NAME_CONVENTION", string(IPName)), "The node naming convention used by the AWS cloud provider. DEPRECATION WARNING: this field may be deprecated at any time")
	f.BoolVar(&opts.AWSENILimitedPodDensity, "aws-eni-limited-pod-density", env.WithDefaultBool("AWS_ENI_LIMITED_POD_DENSITY", true), "Indicates whether new nodes should use ENI-based pod density. DEPRECATED: Use `.spec.kubeletConfiguration.maxPods` to set pod density on a per-provisioner basis")
	f.StringVar(&opts.AWSDefaultInstanceProfile, "aws-default-instance-profile", env.WithDefaultString("AWS_DEFAULT_INSTANCE_PROFILE", ""), "The default instance profile to use when provisioning nodes in AWS")
	f.BoolVar(&opts.AWSEnablePodENI, "aws-enable-pod-eni", env.WithDefaultBool("AWS_ENABLE_POD_ENI", false), "If true then instances that support pod ENI will report a vpc.amazonaws.com/pod-eni resource")
	f.BoolVar(&opts.AWSIsolatedVPC, "aws-isolated-vpc", env.WithDefaultBool("AWS_ISOLATED_VPC", false), "If true then assume we can't reach AWS services which don't have a VPC endpoint. This also has the effect of disabling look-ups to the AWS pricing endpoint.")
	return opts
}

// MustParse reads the user passed flags, environment variables, and default values.
// Options are valided and panics if an error is returned
func (o *Options) MustParse() *Options {
	err := o.Parse(os.Args[1:])

	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if err != nil {
		panic(err)
	}
	if err := o.Validate(); err != nil {
		panic(err)
	}
	return o
}

func (o Options) Validate() (err error) {
	err = multierr.Append(err, o.validateEndpoint())
	if o.ClusterName == "" {
		err = multierr.Append(err, fmt.Errorf("CLUSTER_NAME is required"))
	}
	awsNodeNameConvention := AWSNodeNameConvention(o.AWSNodeNameConvention)
	if awsNodeNameConvention != IPName && awsNodeNameConvention != ResourceName {
		err = multierr.Append(err, fmt.Errorf("aws-node-name-convention may only be either ip-name or resource-name"))
	}
	return err
}

func (o Options) validateEndpoint() error {
	endpoint, err := url.Parse(o.ClusterEndpoint)
	// url.Parse() will accept a lot of input without error; make
	// sure it's a real URL
	if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
		return fmt.Errorf("\"%s\" not a valid CLUSTER_ENDPOINT URL", o.ClusterEndpoint)
	}
	return nil
}

func (o Options) GetAWSNodeNameConvention() AWSNodeNameConvention {
	return AWSNodeNameConvention(o.AWSNodeNameConvention)
}
