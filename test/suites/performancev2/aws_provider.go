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

package performancev2

import (
	_ "embed"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/test/suites/performancev2/env"
	"sigs.k8s.io/karpenter/test/suites/performancev2/perf"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

//go:embed calibration/aws.json
var awsCalibration []byte

// AWSAPICalls counts the controller's real AWS API calls in a phase. The KWOK controller fakes launches, so these
// are the describe, SSM and pricing calls that caching is meant to keep flat.
var AWSAPICalls = perf.Metric{Name: "aws_api_calls", Kind: perf.KindResource, Unit: "calls", Upper: true,
	Measures: "AWS SDK requests the leader made during the phase (aws_sdk_go_request_total delta, all services).",
	OnBreach: "More AWS calls for the same work: a cache that stopped hitting (instance types, subnets, AMIs, pricing) or a new call per pod or NodeClaim. The report's counters split it by service and action.",
	Value:    func(r *perf.Result) float64 { return float64(r.Counter("aws_sdk_go_request_total", nil)) }}

// AWS is the AWS provider's KWOK controller (./kwok): the real AWS provider against real instance types,
// pricing, subnets and AMIs, with launches faked. It needs CLUSTER_NAME for the EC2NodeClass selectors.
func AWS(c env.Clients) *perf.Provider {
	cluster := os.Getenv("CLUSTER_NAME")
	return &perf.Provider{
		Name:       "aws-kwok",
		Summary:    "AWS provider's KWOK controller on EKS: real instance types and AWS APIs, faked launches.",
		Client:     c.Client,
		KubeClient: c.KubeClient,
		Controller: perf.Controller{
			Namespace:   "kube-system",
			Selector:    labels.SelectorFromSet(map[string]string{"app.kubernetes.io/name": "karpenter"}),
			MetricsPort: "8080",
		},
		NodeClass: func(*perf.Scenario) client.Object {
			return test.EC2NodeClass(v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{
				Role:                       fmt.Sprintf("KarpenterNodeRole-%s", cluster),
				AMISelectorTerms:           []v1.AMISelectorTerm{{Alias: "al2023@latest"}},
				SubnetSelectorTerms:        []v1.SubnetSelectorTerm{{Tags: map[string]string{"karpenter.sh/discovery": cluster}}},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"karpenter.sh/discovery": cluster}}},
			}})
		},
		NodePool: func(_ *perf.Scenario, nc client.Object) *karpv1.NodePool {
			np := env.NodePool(nc)
			coretest.ReplaceRequirements(np,
				karpv1.NodeSelectorRequirementWithMinValues{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"linux"}},
				karpv1.NodeSelectorRequirementWithMinValues{Key: karpv1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.CapacityTypeOnDemand}},
				karpv1.NodeSelectorRequirementWithMinValues{Key: v1.LabelInstanceCategory, Operator: corev1.NodeSelectorOpIn, Values: []string{"c", "m", "r"}},
				karpv1.NodeSelectorRequirementWithMinValues{Key: v1.LabelInstanceGeneration, Operator: corev1.NodeSelectorOpGt, Values: []string{"4"}},
				karpv1.NodeSelectorRequirementWithMinValues{Key: v1.LabelInstanceCPU, Operator: corev1.NodeSelectorOpLt, Values: []string{"32"}},
			)
			np.Spec.Limits = karpv1.Limits{}
			np.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
			np.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("30s")
			np.Spec.Disruption.Budgets = []karpv1.Budget{{Nodes: "100%"}}
			np.Spec.Template.Spec.ExpireAfter = karpv1.MustParseNillableDuration("Never")
			return np
		},
		Metrics:     []perf.Metric{AWSAPICalls},
		Capture:     []string{"aws_sdk_go_"},
		ReproEnv:    []string{"CLUSTER_NAME"},
		Calibration: perf.MustLoadCalibration(awsCalibration),
		Levels: perf.Levels{
			"pods/scaleOut":        {},
			"pods/consolidation":   {},
			"nodepools/scheduling": {},
			"nodepools/startup":    {},
			"topology/scaleOut":    {},
		},
	}
}
