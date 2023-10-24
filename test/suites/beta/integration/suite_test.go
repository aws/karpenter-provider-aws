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

package integration_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment
var nodeClass *v1beta1.EC2NodeClass
var nodePool *corev1beta1.NodePool

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Integration")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
		Spec: v1beta1.EC2NodeClassSpec{
			AMIFamily: &v1beta1.AMIFamilyAL2,
			SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				},
			},
			SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				},
			},
			Role: fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName),
		},
	})
	nodePool = coretest.NodePool(corev1beta1.NodePool{
		Spec: corev1beta1.NodePoolSpec{
			Template: corev1beta1.NodeClaimTemplate{
				Spec: corev1beta1.NodeClaimSpec{
					NodeClassRef: &corev1beta1.NodeClassReference{
						Name: nodeClass.Name,
					},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelOSStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{string(v1.Linux)},
						},
						{
							Key:      corev1beta1.CapacityTypeLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{corev1beta1.CapacityTypeOnDemand},
						},
						{
							Key:      v1beta1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"c", "m", "r"},
						},
						{
							Key:      v1beta1.LabelInstanceGeneration,
							Operator: v1.NodeSelectorOpGt,
							Values:   []string{"2"},
						},
					},
				},
			},
			Disruption: corev1beta1.Disruption{
				ConsolidateAfter: &corev1beta1.NillableDuration{},
			},
		},
	})
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })
