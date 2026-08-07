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

// celtest previews what a kubelet CEL expression would evaluate to before it is applied to a cluster.
//
// It evaluates expressions through the same code the controller runs (pkg/cel plus
// instancetype.PreviewKubeletExpressions), so the value it prints is the value a real launch would compute,
// including the per-field unit suffix and the drop rules for failed or out-of-range results.
//
// Offline (no AWS credentials or network) — supply the instance type's inputs directly:
//
//	go run ./hack/tools/celtest --max-pods-expr 'min(max_pods, vcpus * 10)' \
//	  --vcpus 2 --memory-mib 7808 --default-enis 3 --ips-per-eni 10
//
// Against a real instance type — looks the inputs up with a read-only DescribeInstanceTypes call:
//
//	go run ./hack/tools/celtest --kube-reserved cpu='max(60, vcpus * 30)' \
//	  --instance-type m5.large --region us-west-2
//
// Multiple instance types and multiple fields can be previewed at once, which is the quickest way to see how
// an expression behaves across a fleet:
//
//	go run ./hack/tools/celtest --instance-type m5.large,c6g.16xlarge,t3.micro --region us-west-2 \
//	  --max-pods-expr 'vcpus * 8' --kube-reserved memory='memory_mib / 100'
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/intstr"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

// mapFlag collects repeatable key=value flags, e.g. --kube-reserved cpu='vcpus * 30'.
type mapFlag map[string]string

func (m mapFlag) String() string { return fmt.Sprint(map[string]string(m)) }

func (m mapFlag) Set(v string) error {
	key, expr, found := strings.Cut(v, "=")
	if !found {
		return fmt.Errorf("expected key=expression, got %q", v)
	}
	if key == "" {
		return fmt.Errorf("empty resource key in %q", v)
	}
	m[key] = expr
	return nil
}

type config struct {
	maxPodsExpr    string
	kubeReserved   mapFlag
	systemReserved mapFlag
	podsPerCore    int
	amiFamily      string
	reservedENIs   int

	instanceTypes string
	region        string

	// Offline inputs, used when --instance-type is not supplied.
	vcpus       int64
	memoryMiB   int64
	defaultENIs int64
	ipsPerENI   int64
}

func main() {
	cfg := &config{kubeReserved: mapFlag{}, systemReserved: mapFlag{}}
	flag.StringVar(&cfg.maxPodsExpr, "max-pods-expr", "", "CEL expression for the kubelet maxPods field")
	flag.Var(cfg.kubeReserved, "kube-reserved", "kubeReserved entry as key=expression (repeatable), e.g. cpu='max(60, vcpus * 30)'")
	flag.Var(cfg.systemReserved, "system-reserved", "systemReserved entry as key=expression (repeatable)")
	flag.IntVar(&cfg.podsPerCore, "pods-per-core", 0, "kubelet podsPerCore, which caps a resolved maxPods when non-zero")
	flag.StringVar(&cfg.amiFamily, "ami-family", v1.AMIFamilyAL2023, "AMI family, which determines the default max_pods calculation")
	flag.IntVar(&cfg.reservedENIs, "reserved-enis", 0, "reserved ENIs, matching the controller's --reserved-enis setting")

	flag.StringVar(&cfg.instanceTypes, "instance-type", "", "comma-separated instance types to look up via a read-only DescribeInstanceTypes call; omit to run fully offline")
	flag.StringVar(&cfg.region, "region", "", "AWS region for the instance type lookup (defaults to the region from the ambient AWS config)")

	flag.Int64Var(&cfg.vcpus, "vcpus", 0, "offline: vCPU count")
	flag.Int64Var(&cfg.memoryMiB, "memory-mib", 0, "offline: memory in MiB")
	flag.Int64Var(&cfg.defaultENIs, "default-enis", 0, "offline: ENIs on the default network card")
	flag.Int64Var(&cfg.ipsPerENI, "ips-per-eni", 0, "offline: IPv4 addresses per ENI")
	flag.Parse()

	if err := run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config) error {
	kc := kubeletConfiguration(cfg)
	if !kc.HasExpressions() {
		return fmt.Errorf("no expressions to preview: pass --max-pods-expr, --kube-reserved, or --system-reserved\n" +
			"note that a value which parses as a resource quantity (e.g. \"100Mi\") is static and never evaluated")
	}
	ctx = injectOptions(ctx, cfg)

	infos, err := instanceTypeInfos(ctx, cfg)
	if err != nil {
		return err
	}
	celEnv, err := kubeletcel.NewEnvironment()
	if err != nil {
		return fmt.Errorf("building CEL environment: %w", err)
	}
	// A compile-only pass first, so a syntax or type error is reported once against the expression itself
	// rather than repeated per instance type.
	if err := validate(celEnv, kc); err != nil {
		return err
	}
	amiFamily := amifamily.GetAMIFamily(cfg.amiFamily, &amifamily.Options{})

	previews := lo.Map(infos, func(info ec2types.InstanceTypeInfo, _ int) instancetype.KubeletExpressionPreview {
		return instancetype.PreviewKubeletExpressions(ctx, celEnv, info, kc, amiFamily, nil)
	})
	report(os.Stdout, previews)
	// Exit non-zero when any expression would be dropped, so this is usable as a pre-apply check in a script.
	if lo.SomeBy(previews, func(p instancetype.KubeletExpressionPreview) bool {
		return lo.SomeBy(p.Expressions, func(e instancetype.PreviewedExpression) bool { return !e.Applied })
	}) {
		return fmt.Errorf("at least one expression would not be applied; see the reasons above")
	}
	return nil
}

// kubeletConfiguration assembles the KubeletConfiguration the preview evaluates, mirroring what a user would
// write in an EC2NodeClass.
func kubeletConfiguration(cfg *config) *v1.KubeletConfiguration {
	kc := &v1.KubeletConfiguration{}
	if cfg.maxPodsExpr != "" {
		kc.MaxPods = lo.ToPtr(intstr.FromString(cfg.maxPodsExpr))
	}
	if len(cfg.kubeReserved) > 0 {
		kc.KubeReserved = cfg.kubeReserved
	}
	if len(cfg.systemReserved) > 0 {
		kc.SystemReserved = cfg.systemReserved
	}
	if cfg.podsPerCore > 0 {
		kc.PodsPerCore = lo.ToPtr(int32(cfg.podsPerCore))
	}
	return kc
}

// injectOptions puts the operator options the evaluation path reads onto the context: ReservedENIs feeds the
// default max_pods calculation, and the NodeClassCEL feature gate is forced on because resolveMaxPods
// declines to evaluate a maxPods expression when it is off — previewing CEL is the tool's entire purpose.
func injectOptions(ctx context.Context, cfg *config) context.Context {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	return options.ToContext(ctx, test.Options(test.OptionsFields{
		ReservedENIs: lo.ToPtr(cfg.reservedENIs),
		FeatureGates: test.FeatureGates{NodeClassCEL: lo.ToPtr(true)},
	}))
}

// validate compile-checks every expression up front. Compilation catches syntax errors, unknown variables, and
// a non-numeric result type; per-instance-type evaluation failures still surface later in the report.
func validate(celEnv *kubeletcel.CELEnvironment, kc *v1.KubeletConfiguration) error {
	if kc.MaxPods != nil && kc.MaxPods.Type == intstr.String {
		if err := celEnv.ValidateExpression(kc.MaxPods.StrVal); err != nil {
			return fmt.Errorf("maxPods: %w", err)
		}
	}
	for _, m := range []struct {
		field string
		m     map[string]string
	}{
		{"kubeReserved", kc.KubeReserved},
		{"systemReserved", kc.SystemReserved},
	} {
		for _, key := range sortedKeys(m.m) {
			if err := celEnv.ValidateExpression(m.m[key]); err != nil {
				return fmt.Errorf("%s[%s]: %w", m.field, key, err)
			}
		}
	}
	return nil
}

// instanceTypeInfos returns the instance types to preview against: either looked up from EC2 when
// --instance-type is set, or a single synthetic instance type built from the offline flags.
func instanceTypeInfos(ctx context.Context, cfg *config) ([]ec2types.InstanceTypeInfo, error) {
	if cfg.instanceTypes == "" {
		return []ec2types.InstanceTypeInfo{offlineInstanceTypeInfo(cfg)}, nil
	}
	names := lo.Compact(lo.Map(strings.Split(cfg.instanceTypes, ","), func(s string, _ int) string {
		return strings.TrimSpace(s)
	}))
	if len(names) == 0 {
		return nil, fmt.Errorf("--instance-type was set but contained no instance type names")
	}
	return describeInstanceTypes(ctx, cfg.region, names)
}

// offlineInstanceTypeInfo builds an InstanceTypeInfo from the offline flags. Only the fields the CEL variables
// are derived from are populated, which is all buildCELVars and the default max_pods calculation read.
func offlineInstanceTypeInfo(cfg *config) ec2types.InstanceTypeInfo {
	return ec2types.InstanceTypeInfo{
		InstanceType: ec2types.InstanceType("offline"),
		VCpuInfo:     &ec2types.VCpuInfo{DefaultVCpus: lo.ToPtr(int32(cfg.vcpus))},
		MemoryInfo:   &ec2types.MemoryInfo{SizeInMiB: lo.ToPtr(cfg.memoryMiB)},
		NetworkInfo: &ec2types.NetworkInfo{
			DefaultNetworkCardIndex:   lo.ToPtr(int32(0)),
			Ipv4AddressesPerInterface: lo.ToPtr(int32(cfg.ipsPerENI)),
			NetworkCards: []ec2types.NetworkCardInfo{{
				MaximumNetworkInterfaces: lo.ToPtr(int32(cfg.defaultENIs)),
			}},
		},
	}
}

// describeInstanceTypes fetches the real inputs for the named instance types. This is the only network call the
// tool makes and it is strictly read-only.
func describeInstanceTypes(ctx context.Context, region string, names []string) ([]ec2types.InstanceTypeInfo, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	if awsCfg.Region == "" {
		return nil, fmt.Errorf("no AWS region configured: pass --region or set AWS_REGION")
	}
	out, err := ec2.NewFromConfig(awsCfg).DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: lo.Map(names, func(n string, _ int) ec2types.InstanceType { return ec2types.InstanceType(n) }),
	})
	if err != nil {
		return nil, fmt.Errorf("describing instance types in %s: %w", awsCfg.Region, err)
	}
	if len(out.InstanceTypes) == 0 {
		return nil, fmt.Errorf("no instance types matched %s in %s", strings.Join(names, ", "), awsCfg.Region)
	}
	// Report anything EC2 didn't return rather than silently previewing a subset.
	found := lo.SliceToMap(out.InstanceTypes, func(i ec2types.InstanceTypeInfo) (string, bool) {
		return string(i.InstanceType), true
	})
	if missing := lo.Filter(names, func(n string, _ int) bool { return !found[n] }); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "warning: not found in %s, skipping: %s\n", awsCfg.Region, strings.Join(missing, ", "))
	}
	infos := out.InstanceTypes
	sort.Slice(infos, func(i, j int) bool { return infos[i].InstanceType < infos[j].InstanceType })
	return infos, nil
}

// report prints, per instance type, the resolved input variables followed by each expression's outcome. The
// inputs are shown because a surprising result is usually a surprising input rather than a bad expression.
func report(out *os.File, previews []instancetype.KubeletExpressionPreview) {
	for i, p := range previews {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "%s\n", p.InstanceType)
		fmt.Fprintf(out, "  inputs: %s\n", formatVars(p.MaxPodsVars))
		if p.ReservedVarsBuilt && p.ReservedVars.MaxPods != p.MaxPodsVars.MaxPods {
			// Worth calling out explicitly: max_pods means the default in a maxPods expression and the
			// resolved value in a kubeReserved/systemReserved expression.
			fmt.Fprintf(out, "  note:   max_pods is %d in the maxPods expression (the AMI-family default) but %d in the reserved expressions (the resolved maxPods)\n",
				p.MaxPodsVars.MaxPods, p.ReservedVars.MaxPods)
		}
		w := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
		fmt.Fprintln(w, "  FIELD\tEXPRESSION\tRESULT")
		for _, e := range p.Expressions {
			result := e.Value
			if !e.Applied {
				result = "DROPPED: " + e.Err.Error()
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\n", e.Field, e.Expression, result)
		}
		w.Flush()
	}
}

func formatVars(v kubeletcel.InstanceTypeVars) string {
	return fmt.Sprintf("vcpus=%d memory_mib=%d default_enis=%d ips_per_eni=%d max_pods=%d instance_type=%q",
		v.VCPUs, v.MemoryMiB, v.DefaultENIs, v.IPsPerENI, v.MaxPods, v.InstanceType)
}

func sortedKeys(m map[string]string) []string {
	keys := lo.Keys(m)
	sort.Strings(keys)
	return keys
}
