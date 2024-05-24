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

package instancetype_test

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	controllersinstancetype "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *controllersinstancetype.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceType")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	controller = controllersinstancetype.NewController(awsEnv.InstanceTypesProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InstanceType", func() {
	It("should update instance type date with response from the DescribeInstanceTypes API", func() {
		ec2InstanceTypes := fake.MakeInstances()
		ec2Offerings := fake.MakeInstanceOfferings(ec2InstanceTypes)
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: ec2InstanceTypes,
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: ec2Offerings,
		})

		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, &corev1beta1.KubeletConfiguration{}, &v1beta1.EC2NodeClass{
			Status: v1beta1.EC2NodeClassStatus{
				Subnets: []v1beta1.Subnet{
					{
						ID:   "subnet-test1",
						Zone: "test-zone-1a",
					},
					{
						ID:   "subnet-test2",
						Zone: "test-zone-1b",
					},
					{
						ID:   "subnet-test3",
						Zone: "test-zone-1c",
					},
				},
			},
		})
		Expect(err).To(BeNil())
		for i := range instanceTypes {
			Expect(instanceTypes[i].Name).To(Equal(lo.FromPtr(ec2InstanceTypes[i].InstanceType)))
		}
	})
	It("should update instance type offering date with response from the DescribeInstanceTypesOfferings API", func() {
		ec2InstanceTypes := fake.MakeInstances()
		ec2Offerings := fake.MakeInstanceOfferings(ec2InstanceTypes)
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: ec2InstanceTypes,
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: ec2Offerings,
		})

		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, &corev1beta1.KubeletConfiguration{}, &v1beta1.EC2NodeClass{
			Status: v1beta1.EC2NodeClassStatus{
				Subnets: []v1beta1.Subnet{
					{
						ID:   "subnet-test1",
						Zone: "test-zone-1a",
					},
					{
						ID:   "subnet-test2",
						Zone: "test-zone-1b",
					},
					{
						ID:   "subnet-test3",
						Zone: "test-zone-1c",
					},
				},
			},
		})
		Expect(err).To(BeNil())

		Expect(len(instanceTypes)).To(BeNumerically("==", len(ec2InstanceTypes)))
		for x := range instanceTypes {
			offering, found := lo.Find(ec2Offerings, func(off *ec2.InstanceTypeOffering) bool {
				return instanceTypes[x].Name == lo.FromPtr(off.InstanceType)
			})
			Expect(found).To(BeTrue())
			for y := range instanceTypes[x].Offerings {
				Expect(instanceTypes[x].Offerings[y].Requirements.Get(v1.LabelTopologyZone).Any()).To(Equal(lo.FromPtr(offering.Location)))
			}
		}
	})
	It("should not update instance type date with response from the DescribeInstanceTypes API is empty", func() {
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{})
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		_, err := awsEnv.InstanceTypesProvider.List(ctx, &corev1beta1.KubeletConfiguration{}, &v1beta1.EC2NodeClass{})
		Expect(err).ToNot(BeNil())
	})
	It("should not update instance type offering date with response from the DescribeInstanceTypesOfferings API", func() {
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{})
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		_, err := awsEnv.InstanceTypesProvider.List(ctx, &corev1beta1.KubeletConfiguration{}, &v1beta1.EC2NodeClass{})
		Expect(err).ToNot(BeNil())
	})
})
