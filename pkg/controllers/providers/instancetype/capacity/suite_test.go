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

package capacity_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpcloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	controllersinstancetypecapacity "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instancetype/capacity"
	"github.com/aws/karpenter-provider-aws/pkg/fake"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *controllersinstancetypecapacity.Controller

var nodeClass *v1.EC2NodeClass
var nodeClaim *karpv1.NodeClaim
var node *corev1.Node

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CapacityCache")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(coretest.NodeClaimProviderIDFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
		VMMemoryOverheadPercent: lo.ToPtr[float64](0.075),
	}))
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	nodeClass = test.EC2NodeClass()
	nodeClaim = coretest.NodeClaim()
	node = coretest.Node()
	cloudProvider := cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider)
	controller = controllersinstancetypecapacity.NewController(env.Client, cloudProvider, awsEnv.InstanceTypesProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
	ec2InstanceTypeInfo := fake.MakeInstances()
	ec2Offerings := fake.MakeInstanceOfferings(ec2InstanceTypeInfo)
	awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
		InstanceTypes: ec2InstanceTypeInfo,
	})
	awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
		InstanceTypeOfferings: ec2Offerings,
	})
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("CapacityCache", func() {
	BeforeEach(func() {
		ExpectApplied(ctx, env.Client, nodeClass)

		node = coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					corev1.LabelInstanceTypeStable:   "t3.medium",
					karpv1.NodeRegisteredLabelKey:    "true",
					"karpenter.k8s.aws/ec2nodeclass": nodeClass.Name,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", 3840)),
			},
		})
		ExpectApplied(ctx, env.Client, node)

		nodeClaim = &karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-nodeclaim",
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
				Requirements: make([]karpv1.NodeSelectorRequirementWithMinValues, 0),
			},
			Status: karpv1.NodeClaimStatus{
				NodeName: node.Name,
				ImageID:  nodeClass.Status.AMIs[0].ID,
			},
		}
		ExpectApplied(ctx, env.Client, nodeClaim)
	})
	It("should update discovered capacity based on existing nodes", func() {
		ExpectObjectReconciled(ctx, env.Client, controller, node)
		instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		i, ok := lo.Find(instanceTypes, func(i *karpcloudprovider.InstanceType) bool {
			return i.Name == "t3.medium"
		})
		Expect(ok).To(BeTrue())
		Expect(i.Capacity.Memory().Value()).To(Equal(node.Status.Capacity.Memory().Value()), "Expected capacity to match discovered node capacity")
	})
	It("should use VM_MEMORY_OVERHEAD_PERCENT calculation after AMI update", func() {
		ExpectObjectReconciled(ctx, env.Client, controller, node)

		// Update NodeClass AMI and list instance-types. Cached values from prior AMI should no longer be used.
		nodeClass.Status.AMIs[0].ID = "ami-new-test-id"
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectObjectReconciled(ctx, env.Client, controller, node)
		instanceTypesNoCache, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		i, ok := lo.Find(instanceTypesNoCache, func(i *karpcloudprovider.InstanceType) bool {
			return i.Name == "t3.medium"
		})
		Expect(ok).To(BeTrue())

		// Calculate memory capacity based on VM_MEMORY_OVERHEAD_PERCENT and output from DescribeInstanceType
		mem := resources.Quantity(fmt.Sprintf("%dMi", 8192)) // Reported memory from fake.MakeInstances()
		mem.Sub(resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(mem.Value())*options.FromContext(ctx).VMMemoryOverheadPercent/1024/1024)))))
		Expect(i.Capacity.Memory().Value()).To(Equal(mem.Value()), "Expected capacity to match VMMemoryOverheadPercent calculation")
	})
})
