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
	"flag"
	"fmt"
	"net/url"

	"github.com/aws/karpenter/pkg/utils/env"
	"go.uber.org/multierr"
)

type AWSNodeNameConvention string

const (
	IPName       AWSNodeNameConvention = "ip-name"
	ResourceName AWSNodeNameConvention = "resource-name"
)

func MustParse() Options {
	opts := Options{}
	flag.StringVar(&opts.ClusterName, "cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "The kubernetes cluster name for resource discovery")
	flag.StringVar(&opts.ClusterEndpoint, "cluster-endpoint", env.WithDefaultString("CLUSTER_ENDPOINT", ""), "The external kubernetes cluster endpoint for new nodes to connect with")
	flag.StringVar(&opts.KarpenterService, "karpenter-service", env.WithDefaultString("KARPENTER_SERVICE", ""), "The Karpenter Service name for the dynamic webhook certificate")
	flag.IntVar(&opts.MetricsPort, "metrics-port", env.WithDefaultInt("METRICS_PORT", 8080), "The port the metric endpoint binds to for operating metrics about the controller itself")
	flag.IntVar(&opts.HealthProbePort, "health-probe-port", env.WithDefaultInt("HEALTH_PROBE_PORT", 8081), "The port the health probe endpoint binds to for reporting controller health")
	flag.IntVar(&opts.WebhookPort, "port", 8443, "The port the webhook endpoint binds to for validation and mutation of resources")
	flag.IntVar(&opts.KubeClientQPS, "kube-client-qps", env.WithDefaultInt("KUBE_CLIENT_QPS", 200), "The smoothed rate of qps to kube-apiserver")
	flag.IntVar(&opts.KubeClientBurst, "kube-client-burst", env.WithDefaultInt("KUBE_CLIENT_BURST", 300), "The maximum allowed burst of queries to the kube-apiserver")
	flag.StringVar(&opts.AWSNodeNameConvention, "aws-node-name-convention", env.WithDefaultString("AWS_NODE_NAME_CONVENTION", string(IPName)), "The node naming convention used by the AWS cloud provider. DEPRECATION WARNING: this field may be deprecated at any time")
	flag.BoolVar(&opts.AWSENILimitedPodDensity, "aws-eni-limited-pod-density", env.WithDefaultBool("AWS_ENI_LIMITED_POD_DENSITY", true), "Indicates whether new nodes should use ENI-based pod density")
	flag.StringVar(&opts.AWSDefaultInstanceProfile, "aws-default-instance-profile", env.WithDefaultString("AWS_DEFAULT_INSTANCE_PROFILE", ""), "The default instance profile to use when provisioning nodes in AWS")
	flag.Parse()
	if err := opts.Validate(); err != nil {
		panic(err)
	}
	return opts
}

// Options for running this binary
type Options struct {
	ClusterName               string
	ClusterEndpoint           string
	KarpenterService          string
	MetricsPort               int
	HealthProbePort           int
	WebhookPort               int
	KubeClientQPS             int
	KubeClientBurst           int
	AWSNodeNameConvention     string
	AWSENILimitedPodDensity   bool
	AWSDefaultInstanceProfile string
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
