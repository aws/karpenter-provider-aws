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
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/utils/env"
)

func init() {
	coreoptions.Injectables = append(coreoptions.Injectables, &Options{})
}

type optionsKey struct{}

type Options struct {
	AssumeRoleARN                          string
	AssumeRoleDuration                     time.Duration
	ClusterCABundle                        string
	ClusterName                            string
	ClusterEndpoint                        string
	IsolatedVPC                            bool
	VMMemoryOverheadPercent                float64
	InterruptionQueue                      string
	ReservedENIs                           int
	PerInstanceTypeVMMemoryOverheadPercent PerInstanceTypeVMMemoryOverheadPercent
}

func (o *Options) AddFlags(fs *coreoptions.FlagSet) {
	fs.StringVar(&o.AssumeRoleARN, "assume-role-arn", env.WithDefaultString("ASSUME_ROLE_ARN", ""), "Role to assume for calling AWS services.")
	fs.DurationVar(&o.AssumeRoleDuration, "assume-role-duration", env.WithDefaultDuration("ASSUME_ROLE_DURATION", 15*time.Minute), "Duration of assumed credentials in minutes. Default value is 15 minutes. Not used unless aws.assumeRole set.")
	fs.StringVar(&o.ClusterCABundle, "cluster-ca-bundle", env.WithDefaultString("CLUSTER_CA_BUNDLE", ""), "Cluster CA bundle for nodes to use for TLS connections with the API server. If not set, this is taken from the controller's TLS configuration.")
	fs.StringVar(&o.ClusterName, "cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "[REQUIRED] The kubernetes cluster name for resource discovery.")
	fs.StringVar(&o.ClusterEndpoint, "cluster-endpoint", env.WithDefaultString("CLUSTER_ENDPOINT", ""), "The external kubernetes cluster endpoint for new nodes to connect with. If not specified, will discover the cluster endpoint using DescribeCluster API.")
	fs.BoolVarWithEnv(&o.IsolatedVPC, "isolated-vpc", "ISOLATED_VPC", false, "If true, then assume we can't reach AWS services which don't have a VPC endpoint. This also has the effect of disabling look-ups to the AWS on-demand pricing endpoint.")
	fs.Float64Var(&o.VMMemoryOverheadPercent, "vm-memory-overhead-percent", env.WithDefaultFloat64("VM_MEMORY_OVERHEAD_PERCENT", 0.075), "The VM memory overhead as a percent that will be subtracted from the total memory for all instance types.")
	fs.Var(&o.PerInstanceTypeVMMemoryOverheadPercent, "per-instance-type-vm-memory-overhead-percent", "The VM memory overhead as a percent that will be subtracted from the total memory for specific instance types. For instance types that are not specified, the default vm-memory-overhead-percent will be used instead. The input is comma-separated pairs, ex. 'c6a.large=0.07,c6a.4xlarge=0.05'")
	fs.StringVar(&o.InterruptionQueue, "interruption-queue", env.WithDefaultString("INTERRUPTION_QUEUE", ""), "Interruption queue is the name of the SQS queue used for processing interruption events from EC2. Interruption handling is disabled if not specified. Enabling interruption handling may require additional permissions on the controller service account. Additional permissions are outlined in the docs.")
	fs.IntVar(&o.ReservedENIs, "reserved-enis", env.WithDefaultInt("RESERVED_ENIS", 0), "Reserved ENIs are not included in the calculations for max-pods or kube-reserved. This is most often used in the VPC CNI custom networking setup https://docs.aws.amazon.com/eks/latest/userguide/cni-custom-network.html.")
}

func (o *Options) Parse(fs *coreoptions.FlagSet, args ...string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		return fmt.Errorf("parsing flags, %w", err)
	}
	if err := o.Validate(); err != nil {
		return fmt.Errorf("validating options, %w", err)
	}
	return nil
}

func (o *Options) ToContext(ctx context.Context) context.Context {
	return ToContext(ctx, o)
}

func ToContext(ctx context.Context, opts *Options) context.Context {
	return context.WithValue(ctx, optionsKey{}, opts)
}

func FromContext(ctx context.Context) *Options {
	retval := ctx.Value(optionsKey{})
	if retval == nil {
		return nil
	}
	return retval.(*Options)
}

type PerInstanceTypeVMMemoryOverheadPercent map[string]float64

func (m *PerInstanceTypeVMMemoryOverheadPercent) String() string {
	return fmt.Sprintf("%v", *m)
}

func (m *PerInstanceTypeVMMemoryOverheadPercent) Set(value string) error {
	*m = make(map[string]float64)
	instanceTypes := strings.Split(value, ",")
	for _, instanceTypeAndOverhead := range instanceTypes {
		parts := strings.Split(instanceTypeAndOverhead, "=")
		if len(parts) != 2 {
			return fmt.Errorf("invalid instance type vm memory overhead format: %s", instanceTypeAndOverhead)
		}
		instanceType := parts[0]
		vmMemoryOverheadPercent, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return fmt.Errorf("invalid value: %s", vmMemoryOverheadPercent)
		}
		(*m)[instanceType] = vmMemoryOverheadPercent
	}
	return nil
}
