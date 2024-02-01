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
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Utilization", Label(debug.NoWatch), Label(debug.NoEvents), func() {
	It("should provision one pod per node", func() {
		test.ReplaceRequirements(nodePool,
			corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"t3.small"},
				},
			},
			corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpExists,
				},
			},
		)
		deployment := test.Deployment(test.DeploymentOptions{
			Replicas:   100,
			PodOptions: test.PodOptions{ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1.5")}}}})

		env.ExpectCreated(nodeClass, nodePool, deployment)
		env.EventuallyExpectHealthyPodCountWithTimeout(time.Minute*10, labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
		env.ExpectCreatedNodeCount("==", int(*deployment.Spec.Replicas)) // One pod per node enforced by instance size
	})
})
