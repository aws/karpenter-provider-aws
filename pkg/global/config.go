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

package global

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	coreglobal "sigs.k8s.io/karpenter/pkg/global"
	"sigs.k8s.io/karpenter/pkg/utils/env"
)

var Config config

type config struct {
	AssumeRoleARN           string
	AssumeRoleDuration      time.Duration
	ClusterCABundle         string
	ClusterName             string
	ClusterEndpoint         string
	IsolatedVPC             bool
	VMMemoryOverheadPercent float64
	InterruptionQueue       string
	ReservedENIs            int
}

func AddFlags(fs *coreglobal.FlagSet) {
	fs.StringVar(&Config.AssumeRoleARN, "assume-role-arn", env.WithDefaultString("ASSUME_ROLE_ARN", ""), "Role to assume for calling AWS services.")
	fs.DurationVar(&Config.AssumeRoleDuration, "assume-role-duration", env.WithDefaultDuration("ASSUME_ROLE_DURATION", 15*time.Minute), "Duration of assumed credentials in minutes. Default value is 15 minutes. Not used unless aws.assumeRole set.")
	fs.StringVar(&Config.ClusterCABundle, "cluster-ca-bundle", env.WithDefaultString("CLUSTER_CA_BUNDLE", ""), "Cluster CA bundle for nodes to use for TLS connections with the API server. If not set, this is taken from the controller's TLS configuration.")
	fs.StringVar(&Config.ClusterName, "cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "[REQUIRED] The kubernetes cluster name for resource discovery.")
	fs.StringVar(&Config.ClusterEndpoint, "cluster-endpoint", env.WithDefaultString("CLUSTER_ENDPOINT", ""), "The external kubernetes cluster endpoint for new nodes to connect with. If not specified, will discover the cluster endpoint using DescribeCluster API.")
	fs.BoolVarWithEnv(&Config.IsolatedVPC, "isolated-vpc", "ISOLATED_VPC", false, "If true, then assume we can't reach AWS services which don't have a VPC endpoint. This also has the effect of disabling look-ups to the AWS on-demand pricing endpoint.")
	fs.Float64Var(&Config.VMMemoryOverheadPercent, "vm-memory-overhead-percent", env.WithDefaultFloat64("VM_MEMORY_OVERHEAD_PERCENT", 0.075), "The VM memory overhead as a percent that will be subtracted from the total memory for all instance types.")
	fs.StringVar(&Config.InterruptionQueue, "interruption-queue", env.WithDefaultString("INTERRUPTION_QUEUE", ""), "Interruption queue is disabled if not specified. Enabling interruption handling may require additional permissions on the controller service account. Additional permissions are outlined in the docs.")
	fs.IntVar(&Config.ReservedENIs, "reserved-enis", env.WithDefaultInt("RESERVED_ENIS", 0), "Reserved ENIs are not included in the calculations for max-pods or kube-reserved. This is most often used in the VPC CNI custom networking setup https://docs.aws.amazon.com/eks/latest/userguide/cni-custom-network.html.")
}

func Initialize(args ...string) error {
	fs := coreglobal.NewFlagSet()
	coreglobal.AddFlags(fs)
	AddFlags(fs)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		return fmt.Errorf("parsing flags, %w", err)
	}
	if err := coreglobal.ParseFeatureGates(); err != nil {
		return fmt.Errorf("parsing feature gates, %w", err)
	}
	if err := coreglobal.Config.Validate(); err != nil {
		return fmt.Errorf("validating config, %w", err)
	}
	if err := Config.Validate(); err != nil {
		return fmt.Errorf("validating config, %w", err)
	}
	return nil
}
