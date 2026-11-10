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
	"fmt"
	"os"

	"github.com/samber/lo"
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

// AWSAPICalls counts the controller's real AWS API calls in a phase (launches are faked).
var AWSAPICalls = perf.Metric{Name: "aws_api_calls", Kind: perf.KindResource, Unit: "calls", Upper: true,
	Measures: "AWS SDK requests the leader made during the phase (aws_sdk_go_request_total delta, all services).",
	OnBreach: "More AWS calls for the same work: a cache that stopped hitting (instance types, subnets, AMIs, pricing) or a new call per pod or NodeClaim. The report's counters split it by service and action.",
	Value:    func(r *perf.Result) float64 { return float64(r.Counter("aws_sdk_go_request_total", nil)) }}

// AWS is the AWS provider's KWOK controller (./kwok): the real AWS provider against real instance types,
// pricing, subnets and AMIs, with launches faked. It needs CLUSTER_NAME for the EC2NodeClass selectors, and reads
// KARPENTER_NAMESPACE like kwok/Makefile (default kube-system).
func AWS(c env.Clients) *perf.Provider {
	cluster := os.Getenv("CLUSTER_NAME")
	namespace := lo.CoalesceOrEmpty(os.Getenv("KARPENTER_NAMESPACE"), "kube-system")
	return &perf.Provider{
		Name:       "aws-kwok",
		Summary:    "AWS provider's KWOK controller on EKS: real instance types and AWS APIs, faked launches.",
		Client:     c.Client,
		KubeClient: c.KubeClient,
		Controller: perf.Controller{
			Namespace:   namespace,
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
		// Real AWS consolidation can outlast the scenarios' 20m wait.
		TimeoutScale: 1.5,
		Metrics:      []perf.Metric{AWSAPICalls},
		Capture:      []string{"aws_sdk_go_"},
		ReproEnv:     []string{"CLUSTER_NAME", "KARPENTER_NAMESPACE"},
		Levels:       Levels(),
	}
}

// Levels are Experimental bounds from 5 EKS baselines (tier-xl, 8 CPU / 24Gi controller): the larger of 1.2x the max
// and median + 2.33 sd. Unlisted metrics, pods/consolidation and the noisy ones are measured only.
func Levels() perf.Levels {
	return perf.Levels{
		"pods/scaleOut": {
			perf.PodReady.P95.Experimental(52), perf.PodReady.P99.Experimental(63),
			perf.Nodes.Experimental(185), perf.HeapMB.Experimental(306.2), perf.CPUCores.Experimental(0.948),
		},
		"pods/consolidation": {},
		"nodepools/scheduling": {
			perf.TotalTimeSeconds.Experimental(88), perf.PodReady.P95.Experimental(88),
			perf.HeapMB.Experimental(3281.8), perf.CPUCores.Experimental(3.467),
		},
		"nodepools/startup": {
			perf.StartupSeconds.Experimental(16.2), perf.HeapMB.Experimental(116.7),
		},
		"topology/scaleOut": {
			perf.PodReady.P95.Experimental(87), perf.PodReady.P99.Experimental(102), perf.PodReady.Max.Experimental(104),
			perf.Nodes.Experimental(675), perf.HeapMB.Experimental(565.6), perf.CPUCores.Experimental(1.949),
			AWSAPICalls.Experimental(16),
		},
	}
}
