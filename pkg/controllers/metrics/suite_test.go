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

package metrics_test

import (
	"context"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/object"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/metrics"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var cloudProvider *cloudprovider.CloudProvider
var controller *metrics.Controller
var pricingController *pricing.Controller
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "MetricsController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider)
	controller = metrics.NewController(env.Client, cloudProvider)

	pricingController = pricing.NewController(awsEnv.PricingProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())

	spotPriceHistory, pricingData := fake.GenerateDefaultPriceOutput()
	awsEnv.PricingAPI.GetProductsBehavior.Output.Set(pricingData)
	awsEnv.EC2API.DescribeSpotPriceHistoryBehavior.Output.Set(spotPriceHistory)
	ExpectSingletonReconciled(ctx, pricingController)

	nodePool = coretest.NodePool()
	nodeClass = test.EC2NodeClass(
		v1.EC2NodeClass{
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
		},
	)
	nodePool.Spec.Template.Spec.NodeClassRef = &karpv1.NodeClassReference{
		Group: object.GVK(nodeClass).Group,
		Kind:  object.GVK(nodeClass).Kind,
		Name:  nodeClass.Name,
	}
	nodeClass.StatusConditions().SetTrue(status.ConditionReady)
	_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
	Expect(err).To(BeNil())
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
	Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	awsEnv.Reset()
})

var _ = Describe("MetricsController", func() {
	Context("Availability", func() {
		It("should expose availability metrics for instance types", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectSingletonReconciled(ctx, controller)
			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				for _, of := range it.Offerings {
					metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_available", map[string]string{
						"instance_type": it.Name,
						"capacity_type": of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
						"zone":          of.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					Expect(ok).To(BeTrue())
					Expect(metric).To(Not(BeNil()))
					value := metric.GetGauge().Value
					Expect(aws.ToFloat64(value)).To(BeNumerically("==", lo.Ternary(of.Available, 1, 0)))
				}
			}
		})
		It("should only mark offering as available if the subnets select on it", func() {
			nodeClass.Status.Subnets = []v1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectSingletonReconciled(ctx, controller)

			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))

			availableZones := sets.New[string]()
			for _, it := range instanceTypes {
				for _, of := range it.Offerings {
					metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_available", map[string]string{
						"instance_type": it.Name,
						"capacity_type": of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
						"zone":          of.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					Expect(ok).To(BeTrue())
					Expect(metric).To(Not(BeNil()))
					value := metric.GetGauge().Value
					if of.Zone() == "test-zone-1a" {
						Expect(aws.ToFloat64(value)).To(BeNumerically("==", lo.Ternary(of.Available, 1, 0)))
					} else {
						Expect(aws.ToFloat64(value)).To(BeNumerically("==", 0))
					}
					if aws.ToFloat64(value) != 0 {
						availableZones.Insert(of.Zone())
					}
				}
			}
			Expect(availableZones).To(HaveLen(1))
			Expect(availableZones.UnsortedList()).To(ContainElements("test-zone-1a"))
		})
		It("should mark offerings as available as long as one NodePool marks it as available", func() {
			nodeClass.Status.Subnets = []v1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}
			nodeClass2 := test.EC2NodeClass()
			nodeClass2.Status.Subnets = []v1.Subnet{
				{
					ID:   "subnet-test2",
					Zone: "test-zone-1b",
				},
			}
			nodeClass3 := test.EC2NodeClass()
			nodeClass3.Status.Subnets = []v1.Subnet{
				{
					ID:   "subnet-test3",
					Zone: "test-zone-1c",
				},
			}

			nodePool2 := coretest.NodePool()
			nodePool2.Spec.Template.Spec.NodeClassRef = &karpv1.NodeClassReference{
				Group: object.GVK(nodeClass).Group,
				Kind:  object.GVK(nodeClass).Kind,
				Name:  nodeClass2.Name,
			}
			nodePool3 := coretest.NodePool()
			nodePool3.Spec.Template.Spec.NodeClassRef = &karpv1.NodeClassReference{
				Group: object.GVK(nodeClass).Group,
				Kind:  object.GVK(nodeClass).Kind,
				Name:  nodeClass3.Name,
			}
			ExpectApplied(ctx, env.Client, nodePool, nodePool2, nodePool3, nodeClass, nodeClass2, nodeClass3)
			ExpectSingletonReconciled(ctx, controller)

			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))

			availableZones := sets.New[string]()
			for _, it := range instanceTypes {
				for _, of := range it.Offerings {
					metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_available", map[string]string{
						"instance_type": it.Name,
						"capacity_type": of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
						"zone":          of.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					Expect(ok).To(BeTrue())
					Expect(metric).To(Not(BeNil()))
					value := metric.GetGauge().Value
					if of.Zone() == "test-zone-1a" {
						Expect(aws.ToFloat64(value)).To(BeNumerically("==", lo.Ternary(of.Available, 1, 0)))
					}
					if aws.ToFloat64(value) != 0 {
						availableZones.Insert(of.Zone())
					}
				}
			}
			Expect(availableZones).To(HaveLen(3))
			Expect(availableZones.UnsortedList()).To(ContainElements("test-zone-1a", "test-zone-1b", "test-zone-1c"))
		})
		It("should inject reservation availability when selecting on a capacity reservation", func() {

		})
	})
	Context("Pricing", func() {
		It("should expose pricing metrics for instance types", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectSingletonReconciled(ctx, controller)
			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))
			for _, it := range instanceTypes {
				for _, of := range it.Offerings {
					metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_price_estimate", map[string]string{
						"instance_type": it.Name,
						"capacity_type": of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
						"zone":          of.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					Expect(ok).To(BeTrue())
					Expect(metric).To(Not(BeNil()))
					value := metric.GetGauge().Value
					Expect(aws.ToFloat64(value)).To(BeNumerically("==", of.Price))
				}
			}
		})
		It("should expose pricing metrics for offerings even if no subnets select on it", func() {
			nodeClass.Status.Subnets = []v1.Subnet{
				{
					ID:   "subnet-test1",
					Zone: "test-zone-1a",
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			ExpectSingletonReconciled(ctx, controller)

			instanceTypes, err := awsEnv.InstanceTypesProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(len(instanceTypes)).To(BeNumerically(">", 0))

			for _, it := range instanceTypes {
				for _, of := range it.Offerings {
					metric, ok := FindMetricWithLabelValues("karpenter_cloudprovider_instance_type_offering_price_estimate", map[string]string{
						"instance_type": it.Name,
						"capacity_type": of.Requirements.Get(karpv1.CapacityTypeLabelKey).Any(),
						"zone":          of.Requirements.Get(corev1.LabelTopologyZone).Any(),
					})
					Expect(ok).To(BeTrue())
					Expect(metric).To(Not(BeNil()))
					value := metric.GetGauge().Value

					_, spotFound := awsEnv.PricingProvider.SpotPrice(ec2types.InstanceType(it.Name), of.Zone())
					_, odFound := awsEnv.PricingProvider.OnDemandPrice(ec2types.InstanceType(it.Name))
					if of.CapacityType() == karpv1.CapacityTypeSpot && spotFound ||
						of.CapacityType() == karpv1.CapacityTypeOnDemand && odFound {
						Expect(aws.ToFloat64(value)).To(BeNumerically(">", 0))
					} else {
						Expect(aws.ToFloat64(value)).To(BeNumerically("==", 0))
					}

				}
			}
		})
		It("should inject reservation pricing when selecting on a capacity reservation", func() {

		})
	})
})
