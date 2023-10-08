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

package instance_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("NodeClass/InstanceProvider", func() {
	var nodeClass *v1beta1.EC2NodeClass
	var nodePool *corev1beta1.NodePool
	var nodeClaim *corev1beta1.NodeClaim
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(corev1beta1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1beta1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: corev1beta1.NodeClaimSpec{
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
		})
	})
	It("should return an ICE error when all attempted instance types return an ICE error", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
			{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: v1alpha5.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
			{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: v1alpha5.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })

		// Since all the capacity pools are ICEd. This should return back an ICE error
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())
	})
})
