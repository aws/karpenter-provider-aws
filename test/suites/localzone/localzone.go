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

package localzone

import (
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var Env *aws.Environment
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	return Describe("LocalZone", func() {
		BeforeEach(func() {
			// The majority of local zones do not support GP3. Feature support in local zones can be tracked here:
			// https://aws.amazon.com/about-aws/global-infrastructure/localzones/features/
			NodeClass.Spec.BlockDeviceMappings = append(NodeClass.Spec.BlockDeviceMappings, &v1.BlockDeviceMapping{
				DeviceName: lo.ToPtr("/dev/xvda"),
				EBS: &v1.BlockDevice{
					VolumeSize: func() *resource.Quantity {
						quantity, err := resource.ParseQuantity("80Gi")
						Expect(err).To(BeNil())
						return &quantity
					}(),
					VolumeType: lo.ToPtr("gp2"),
					Encrypted:  lo.ToPtr(false),
				},
			})
			NodePool.Spec.Template.Spec.Requirements = append(NodePool.Spec.Template.Spec.Requirements, karpv1.NodeSelectorRequirementWithMinValues{
				Key:      corev1.LabelTopologyZone,
				Operator: corev1.NodeSelectorOpIn,
				Values: lo.FilterMap(Env.DiscoveredSubnetInfo(), func(info aws.SubnetInfo, _ int) (string, bool) {
					return info.Zone, info.ZoneType == "local-zone"
				}),
			})
		})
		It("should successfully scale up nodes in a local zone", func() {
			nodeCount := 3
			depLabels := map[string]string{
				"foo": "bar",
			}
			dep := test.Deployment(test.DeploymentOptions{
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: depLabels,
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
						TopologyKey:       corev1.LabelHostname,
						MaxSkew:           1,
						WhenUnsatisfiable: corev1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: depLabels,
						},
					}},
				},
				Replicas: int32(nodeCount),
			})
			Env.ExpectCreated(NodeClass, NodePool, dep)
			Env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(depLabels), nodeCount)
			Env.EventuallyExpectNodeCount("==", nodeCount)
		})
	})
}
