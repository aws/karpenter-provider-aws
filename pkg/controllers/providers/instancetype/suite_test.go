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
	"sort"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
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
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
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
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
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

		ExpectSingletonReconciled(ctx, controller)
		instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, &v1.EC2NodeClass{
			Status: v1.EC2NodeClassStatus{
				Subnets: []v1.Subnet{
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
		sort.Slice(instanceTypes, func(i, j int) bool {
			return instanceTypes[i].Name < instanceTypes[j].Name
		})
		sort.Slice(ec2InstanceTypes, func(i, j int) bool {
			return ec2InstanceTypes[i].InstanceType < ec2InstanceTypes[j].InstanceType
		})
		for i := range instanceTypes {
			Expect(instanceTypes[i].Name).To(Equal(string(ec2InstanceTypes[i].InstanceType)))
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

		ExpectSingletonReconciled(ctx, controller)
		instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, &v1.EC2NodeClass{
			Status: v1.EC2NodeClassStatus{
				Subnets: []v1.Subnet{
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
			offering, found := lo.Find(ec2Offerings, func(off ec2types.InstanceTypeOffering) bool {
				return instanceTypes[x].Name == string(off.InstanceType)
			})
			Expect(found).To(BeTrue())
			for y := range instanceTypes[x].Offerings {
				Expect(instanceTypes[x].Offerings[y].Requirements.Get(corev1.LabelTopologyZone).Any()).To(Equal(lo.FromPtr(offering.Location)))
			}
		}
	})
	It("should not update instance type date with response from the DescribeInstanceTypes API is empty", func() {
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{})
		ExpectSingletonReconciled(ctx, controller)
		_, err := awsEnv.InstanceTypesProvider.List(ctx, &v1.EC2NodeClass{})
		Expect(err).ToNot(BeNil())
	})
	It("should not update instance type offering date with response from the DescribeInstanceTypesOfferings API", func() {
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{})
		ExpectSingletonReconciled(ctx, controller)
		_, err := awsEnv.InstanceTypesProvider.List(ctx, &v1.EC2NodeClass{})
		Expect(err).ToNot(BeNil())
	})
})
