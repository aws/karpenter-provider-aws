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

package aws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/scheduling"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
	"github.com/aws/karpenter/pkg/utils/parallel"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/patrickmn/go-cache"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *test.Environment
var launchTemplateCache *cache.Cache
var fakeEC2API *fake.EC2API
var provisioners *provisioning.Controller
var scheduler *scheduling.Controller

const shortenedUnavailableOfferingsTTL = 2 * time.Second

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		ctx = injection.WithOptions(ctx, options.Options{ClusterName: "test-cluster", ClusterEndpoint: "https://test-cluster"})
		launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
		fakeEC2API = &fake.EC2API{}
		subnetProvider := NewSubnetProvider(fakeEC2API)
		instanceTypeProvider := &InstanceTypeProvider{
			ec2api:               fakeEC2API,
			subnetProvider:       subnetProvider,
			cache:                cache.New(InstanceTypesAndZonesCacheTTL, CacheCleanupInterval),
			unavailableOfferings: cache.New(shortenedUnavailableOfferingsTTL, InsufficientCapacityErrorCacheCleanupInterval),
		}
		clientSet := kubernetes.NewForConfigOrDie(e.Config)
		cloudProvider := &CloudProvider{
			subnetProvider:       subnetProvider,
			instanceTypeProvider: instanceTypeProvider,
			instanceProvider: &InstanceProvider{
				fakeEC2API, instanceTypeProvider, subnetProvider, &LaunchTemplateProvider{
					ec2api:                fakeEC2API,
					amiProvider:           NewAMIProvider(&fake.SSMAPI{}, clientSet),
					securityGroupProvider: NewSecurityGroupProvider(fakeEC2API),
					cache:                 launchTemplateCache,
				},
			},
			creationQueue: parallel.NewWorkQueue(CreationQPS, CreationBurst),
		}
		registry.RegisterOrDie(ctx, cloudProvider)
		provisioners = provisioning.NewController(ctx, e.Client, clientSet.CoreV1(), cloudProvider)
		scheduler = scheduling.NewController(e.Client, provisioners)
	})

	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha5.Provisioner
	var provider *v1alpha1.AWS

	BeforeEach(func() {
		provider = &v1alpha1.AWS{
			InstanceProfile: "test-instance-profile",
		}
		provisioner = ProvisionerWithProvider(&v1alpha5.Provisioner{ObjectMeta: metav1.ObjectMeta{Name: v1alpha5.DefaultProvisioner.Name}}, provider)
		provisioner.SetDefaults(ctx)
		fakeEC2API.Reset()
		ExpectCleanedUp(env.Client)
		launchTemplateCache.Flush()
	})

	Context("Reconciliation", func() {
		Context("Specialized Hardware", func() {
			It("should launch instances for Nvidia GPU resource requests", func() {
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
						},
					}),
					// Should pack onto same instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
						},
					}),
					// Should pack onto a separate instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
							Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
						},
					})) {
					ExpectScheduled(ctx, env.Client, pod)
					nodeNames.Insert(ExpectScheduledWithInstanceType(ctx, env.Client, pod, "p3.8xlarge").Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
			It("should launch instances for AWS Neuron resource requests", func() {
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					}),
					// Should pack onto same instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						},
					}),
					// Should pack onto a separate instance
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
						},
					}),
				) {
					nodeNames.Insert(ExpectScheduledWithInstanceType(ctx, env.Client, pod, "inf1.6xlarge").Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
		})
		Context("Insufficient Capacity Error Cache", func() {
			It("should launch instances on second recon attempt with Insufficient Capacity Error Cache fallback", func() {
				fakeEC2API.ShouldTriggerInsufficientCapacity = true
				pods := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					}),
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						},
					}),
				)
				// it should've tried to pack them on a single inf1.6xlarge then hit an insufficient capacity error
				for _, pod := range pods {
					ExpectNotScheduled(ctx, env.Client, pod)
				}
				nodeNames := sets.NewString()
				for _, pod := range ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner, pods...) {
					nodeNames.Insert(ExpectScheduledWithInstanceType(ctx, env.Client, pod, "inf1.2xlarge").Name)
				}
				Expect(nodeNames.Len()).To(Equal(2))
			})
			It("should launch instances on later recon attempt with Insufficient Capacity Error Cache expiry", func() {
				fakeEC2API.ShouldTriggerInsufficientCapacity = true
				pods := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						},
					}),
					test.UnschedulablePod(test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{
							Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
							Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						},
					}),
				)
				// it should've tried to pack them on a single inf1.6xlarge then hit an insufficient capacity error
				for _, pod := range pods {
					ExpectNotScheduled(ctx, env.Client, pod)
				}
				// capacity shortage is over - wait for expiry (N.B. the Karpenter logging will not show the overridden cache expiry in this test context)
				fakeEC2API.ShouldTriggerInsufficientCapacity = false
				Eventually(func(g Gomega) int {
					nodeNames := sets.NewString()
					for _, pod := range ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner, pods...) {
						nodeNames = nodeNames.Insert(ExpectScheduledWithInstanceTypeAndGomega(ctx, env.Client, pod, "inf1.6xlarge", g).Name)
					}
					return len(nodeNames)
				}, shortenedUnavailableOfferingsTTL*2, RequestInterval).Should(Equal(1))
			})
		})
		Context("CapacityType", func() {
			It("should default to on demand", func() {
				pod := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner, test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeOnDemand))
			})
			It("should launch spot capacity if flexible to both spot and on demand", func() {
				provisioner.Spec.Requirements = v1alpha5.Requirements{{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}}}
				pod := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeSpot}}),
				)[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeSpot))
			})
		})
		Context("LaunchTemplates", func() {
			It("should use same launch template for equivalent constraints", func() {
				t1 := v1.Toleration{
					Key:      "Abacus",
					Operator: "Equal",
					Value:    "Zebra",
					Effect:   "NoSchedule",
				}
				t2 := v1.Toleration{
					Key:      "Zebra",
					Operator: "Equal",
					Value:    "Abacus",
					Effect:   "NoSchedule",
				}
				t3 := v1.Toleration{
					Key:      "Boar",
					Operator: "Equal",
					Value:    "Abacus",
					Effect:   "NoSchedule",
				}

				pod1 := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						Tolerations: []v1.Toleration{t1, t2, t3},
					}),
				)[0]
				ExpectScheduled(ctx, env.Client, pod1)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				name1 := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput).LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName

				pod2 := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{
						Tolerations: []v1.Toleration{t2, t3, t1},
					}),
				)[0]

				ExpectScheduled(ctx, env.Client, pod2)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				name2 := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput).LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName
				Expect(name1).To(Equal(name2))
			})

			It("should default to a generated launch template", func() {
				pod := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, provisioner, test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.Version).To(Equal("$Default"))
			})
			It("should allow a launch template to be specified", func() {
				provider.LaunchTemplate = aws.String("test-launch-template")
				pod := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal("test-launch-template"))
				Expect(*launchTemplate.Version).To(Equal("$Default"))
			})
		})
		Context("Subnets", func() {
			It("should default to the cluster's subnets", func() {
				pod := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ContainElements(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-1"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1a")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1b")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1c")},
				))
			})
		})
		Context("Security Groups", func() {
			It("should default to the clusters security groups", func() {
				pod := ExpectProvisioned(ctx, env.Client, scheduler, provisioners, ProvisionerWithProvider(provisioner, provider), test.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-1"),
					aws.String("test-security-group-2"),
					aws.String("test-security-group-3"),
				))
			})
		})
	})
	Context("Defaulting", func() {
		It("should default subnetSelector", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.Deserialize(&provisioner.Spec.Constraints)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.SubnetSelector).To(Equal(map[string]string{"kubernetes.io/cluster/test-cluster": "*"}))
		})
		It("should default securityGroupSelector", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.Deserialize(&provisioner.Spec.Constraints)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.SecurityGroupSelector).To(Equal(map[string]string{"kubernetes.io/cluster/test-cluster": "*"}))
		})
		It("should default requirements", func() {
			provisioner.SetDefaults(ctx)
			Expect(provisioner.Spec.Requirements.CapacityTypes().UnsortedList()).To(ConsistOf(v1alpha1.CapacityTypeOnDemand))
			Expect(provisioner.Spec.Requirements.Architectures().UnsortedList()).To(ConsistOf(v1alpha5.ArchitectureAmd64))
		})
	})
	Context("Validation", func() {
		It("should not panic if provider undefined", func() {
			provisioner.Spec.Provider = nil
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})

		Context("SubnetSelector", func() {
			It("should not allow empty string keys or values", func() {
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SubnetSelector = map[string]string{key: value}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("SecurityGroupSelector", func() {
			It("should not allow empty string keys or values", func() {
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SecurityGroupSelector = map[string]string{key: value}
					provisioner := ProvisionerWithProvider(provisioner, provider)
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("Labels", func() {
			It("should not allow unrecognized labels with the aws label prefix", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support a capacity type label", func() {
				for _, value := range []string{v1alpha1.CapacityTypeOnDemand, v1alpha1.CapacityTypeSpot} {
					provisioner.Spec.Labels = map[string]string{v1alpha5.LabelCapacityType: value}
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
		})
	})
})

func ProvisionerWithProvider(provisioner *v1alpha5.Provisioner, provider *v1alpha1.AWS) *v1alpha5.Provisioner {
	raw, err := json.Marshal(provider)
	Expect(err).ToNot(HaveOccurred())
	provisioner.Spec.Constraints.Provider = &runtime.RawExtension{Raw: raw}
	provisioner.Spec.Limits.Resources = v1.ResourceList{}
	provisioner.Spec.Limits.Resources[v1.ResourceCPU] = *resource.NewScaledQuantity(10, 0)
	return provisioner
}

func InstancesLaunchedFrom(createFleetInputIter <-chan interface{}) int {
	instancesLaunched := 0
	for input := range createFleetInputIter {
		createFleetInput := input.(*ec2.CreateFleetInput)
		instancesLaunched += int(*createFleetInput.TargetCapacitySpecification.TotalTargetCapacity)
	}
	return instancesLaunched
}
