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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	v1alpha1 "github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/binpacking"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/scheduling"
	"github.com/awslabs/karpenter/pkg/test"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	"github.com/awslabs/karpenter/pkg/utils/parallel"
	podutils "github.com/awslabs/karpenter/pkg/utils/pod"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *test.Environment
var launchTemplateCache *cache.Cache
var fakeEC2API *fake.EC2API
var controller reconcile.Reconciler

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
	fakeEC2API = &fake.EC2API{}
	instanceTypeProvider := NewInstanceTypeProvider(fakeEC2API)
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		clientSet := kubernetes.NewForConfigOrDie(e.Config)
		cloudProvider := &CloudProvider{
			instanceTypeProvider: instanceTypeProvider,
			instanceProvider: &InstanceProvider{fakeEC2API, instanceTypeProvider, &LaunchTemplateProvider{
				fakeEC2API,
				NewAMIProvider(&fake.SSMAPI{}, clientSet),
				NewSecurityGroupProvider(fakeEC2API),
				launchTemplateCache,
			},
				NewSubnetProvider(fakeEC2API),
			},
			creationQueue: parallel.NewWorkQueue(CreationQPS, CreationBurst),
		}
		registry.RegisterOrDie(ctx, cloudProvider)
		controller = &allocation.Controller{
			Filter:        &podutils.Filter{KubeClient: e.Client},
			Binder:        &allocation.Binder{KubeClient: e.Client, CoreV1Client: clientSet.CoreV1()},
			Batcher:       allocation.NewBatcher(1*time.Millisecond, 1*time.Millisecond),
			Scheduler:     scheduling.NewScheduler(cloudProvider, e.Client),
			Packer:        binpacking.NewPacker(),
			CloudProvider: cloudProvider,
			KubeClient:    e.Client,
		}
	})

	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha4.Provisioner
	var provider *v1alpha1.AWS

	BeforeEach(func() {
		provider = &v1alpha1.AWS{
			Cluster: v1alpha1.Cluster{
				Name:     "test-cluster",
				Endpoint: "https://test-cluster",
			},
			InstanceProfile: "test-instance-profile",
		}
		provisioner = ProvisionerWithProvider(&v1alpha4.Provisioner{ObjectMeta: metav1.ObjectMeta{Name: v1alpha4.DefaultProvisioner.Name}}, provider)
		provisioner.SetDefaults(ctx)
		fakeEC2API.Reset()
		ExpectCleanedUp(env.Client)
		launchTemplateCache.Flush()
	})

	Context("Reconciliation", func() {
		Context("Specialized Hardware", func() {
			It("should launch instances for Nvidia GPU resource requests", func() {
				// Setup
				pod1 := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
					},
				})
				// Should pack onto same instance
				pod2 := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
					},
				})
				// Should pack onto a separate instance
				pod3 := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
					},
				})
				ExpectCreated(env.Client, provisioner)
				ExpectCreatedWithStatus(env.Client, pod1, pod2, pod3)
				ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
				// Assertions
				scheduled1 := ExpectPodExists(env.Client, pod1.GetName(), pod1.GetNamespace())
				scheduled2 := ExpectPodExists(env.Client, pod2.GetName(), pod2.GetNamespace())
				scheduled3 := ExpectPodExists(env.Client, pod3.GetName(), pod3.GetNamespace())
				Expect(scheduled1.Spec.NodeName).To(Equal(scheduled2.Spec.NodeName))
				Expect(scheduled1.Spec.NodeName).ToNot(Equal(scheduled3.Spec.NodeName))
				ExpectNodeExists(env.Client, scheduled1.Spec.NodeName)
				ExpectNodeExists(env.Client, scheduled3.Spec.NodeName)
				Expect(InstancesLaunchedFrom(fakeEC2API.CalledWithCreateFleetInput.Iter())).To(Equal(2))
				overrides := []*ec2.FleetLaunchTemplateOverridesRequest{}
				for i := range fakeEC2API.CalledWithCreateFleetInput.Iter() {
					overrides = append(overrides, i.(*ec2.CreateFleetInput).LaunchTemplateConfigs[0].Overrides...)
				}
				for _, override := range overrides {
					Expect(*override.InstanceType).To(Equal("p3.8xlarge"))
				}
			})
			It("should launch instances for AWS Neuron resource requests", func() {
				// Setup
				pod1 := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
					},
				})
				// Should pack onto same instance
				pod2 := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
					},
				})
				// Should pack onto a separate instance
				pod3 := test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
						Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
					},
				})
				ExpectCreated(env.Client, provisioner)
				ExpectCreatedWithStatus(env.Client, pod1, pod2, pod3)
				ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
				// Assertions
				scheduled1 := ExpectPodExists(env.Client, pod1.GetName(), pod1.GetNamespace())
				scheduled2 := ExpectPodExists(env.Client, pod2.GetName(), pod2.GetNamespace())
				scheduled3 := ExpectPodExists(env.Client, pod3.GetName(), pod3.GetNamespace())
				Expect(scheduled1.Spec.NodeName).To(Equal(scheduled2.Spec.NodeName))
				Expect(scheduled1.Spec.NodeName).ToNot(Equal(scheduled3.Spec.NodeName))
				ExpectNodeExists(env.Client, scheduled1.Spec.NodeName)
				ExpectNodeExists(env.Client, scheduled3.Spec.NodeName)
				Expect(InstancesLaunchedFrom(fakeEC2API.CalledWithCreateFleetInput.Iter())).To(Equal(2))
				overrides := []*ec2.FleetLaunchTemplateOverridesRequest{}
				for input := range fakeEC2API.CalledWithCreateFleetInput.Iter() {
					overrides = append(overrides, input.(*ec2.CreateFleetInput).LaunchTemplateConfigs[0].Overrides...)
				}
				for _, override := range overrides {
					Expect(*override.InstanceType).To(Equal("inf1.6xlarge"))
				}
			})
		})
		Context("CapacityType", func() {
			It("should default to on demand", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeOnDemand))
			})
			It("should default to a provisioner's specified capacity type", func() {
				// Setup
				provider.CapacityTypes = []string{v1alpha1.CapacityTypeSpot}
				ExpectCreated(env.Client, ProvisionerWithProvider(provisioner, provider))
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeSpot))
			})
			It("should launch spot capacity if flexible to both spot and on demand", func() {
				// Setup
				provider.CapacityTypes = []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}
				ExpectCreated(env.Client, ProvisionerWithProvider(provisioner, provider))
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.CapacityTypeLabel: v1alpha1.CapacityTypeSpot}}),
				)
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeSpot))
			})
			It("should allow a pod to constrain the capacity type", func() {
				// Setup
				provider.CapacityTypes = []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}
				ExpectCreated(env.Client, ProvisionerWithProvider(provisioner, provider))
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.CapacityTypeLabel: v1alpha1.CapacityTypeOnDemand}}),
				)
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeOnDemand))
			})
			It("should not schedule a pod if outside of provisioner constraints", func() {
				// Setup
				provider.CapacityTypes = []string{v1alpha1.CapacityTypeOnDemand}
				ExpectCreated(env.Client, ProvisionerWithProvider(provisioner, provider))
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.CapacityTypeLabel: v1alpha1.CapacityTypeSpot}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
			It("should not schedule a pod with an invalid capacityType", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.CapacityTypeLabel: "unknown"}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
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

				ExpectCreated(env.Client, provisioner)
				pod1 := test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{t1, t2, t3},
				})
				pod2 := test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{t2, t3, t1},
				})
				// Ensure it's on its own node
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, pod1)
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				name1 := *(input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName)
				// Ensure it's on its own node
				pods2 := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, pod2)
				ExpectNodeExists(env.Client, pods2[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input2 := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				name2 := *(input2.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName)
				Expect(name1).To(Equal(name2))
			})

			It("should default to a generated launch template", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.Version).To(Equal("$Default"))
			})
			It("should allow a launch template to be specified", func() {
				// Setup
				provider.LaunchTemplate = aws.String("test-launch-template")
				provisioner = ProvisionerWithProvider(provisioner, provider)
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
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
				// Setup
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-1"), InstanceType: aws.String("m5.large")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
			})
		})
		Context("Security Groups", func() {
			It("should default to the clusters security groups", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
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
			constraints, err := v1alpha1.NewConstraints(&provisioner.Spec.Constraints)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.SubnetSelector).To(Equal(map[string]string{"kubernetes.io/cluster/test-cluster": "*"}))
		})
		It("should default securityGroupSelector", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.NewConstraints(&provisioner.Spec.Constraints)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.SecurityGroupSelector).To(Equal(map[string]string{"kubernetes.io/cluster/test-cluster": "*"}))
		})
		It("should default capacityType", func() {
			provisioner.SetDefaults(ctx)
			constraints, err := v1alpha1.NewConstraints(&provisioner.Spec.Constraints)
			Expect(err).ToNot(HaveOccurred())
			Expect(constraints.CapacityTypes).To(ConsistOf(v1alpha1.CapacityTypeOnDemand))
		})
	})
	Context("Validation", func() {
		Context("Cluster", func() {
			It("should fail if fields are empty", func() {
				for _, cluster := range []v1alpha1.Cluster{
					{Endpoint: "https://test-cluster"},
					{Name: "test-cluster"},
					{},
				} {
					provisioner = ProvisionerWithProvider(provisioner, &v1alpha1.AWS{Cluster: cluster})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
			It("should fail for invalid endpoint", func() {
				for _, endpoint := range []string{
					"http",
					"http:",
					"http://",
					"https",
					"https:",
					"https://",
					"I am a meat popsicle",
					"$(echo foo)",
				} {
					provisioner = ProvisionerWithProvider(provisioner, &v1alpha1.AWS{Cluster: v1alpha1.Cluster{Name: "test-cluster", Endpoint: endpoint}})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
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
			It("should not allow labels with the aws label prefix", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})
		Context("Zones", func() {
			It("should fail if not supported", func() {
				provisioner.Spec.Zones = []string{"unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should succeed if supported", func() {
				fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
					{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
					{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
					{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
				}}
				provisioner.Spec.Zones = []string{
					"test-zone-1a",
					"test-zone-1b",
					"test-zone-1c",
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
		Context("InstanceTypes", func() {
			It("should fail if not supported", func() {
				provisioner.Spec.InstanceTypes = []string{"unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should succeed if supported", func() {
				provisioner.Spec.InstanceTypes = []string{
					"m5.large",
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
		Context("Architecture", func() {
			It("should fail if not supported", func() {
				provisioner.Spec.Architectures = []string{"unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support AMD", func() {
				provisioner.Spec.Architectures = []string{v1alpha4.ArchitectureAmd64}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should support ARM", func() {
				provisioner.Spec.Architectures = []string{v1alpha4.ArchitectureArm64}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
		Context("OperatingSystem", func() {
			It("should fail if not supported", func() {
				provisioner.Spec.OperatingSystems = []string{"unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support linux", func() {
				provisioner.Spec.OperatingSystems = []string{v1alpha4.OperatingSystemLinux}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
		Context("CapacityType", func() {
			It("should fail if not supported", func() {
				provider.CapacityTypes = []string{"unknown"}
				Expect(ProvisionerWithProvider(provisioner, provider).Validate(ctx)).ToNot(Succeed())
			})
			It("should support spot", func() {
				provider.CapacityTypes = []string{"spot"}
				Expect(ProvisionerWithProvider(provisioner, provider).Validate(ctx)).ToNot(Succeed())
			})
			It("should support on demand", func() {
				provider.CapacityTypes = []string{"on demand"}
				Expect(ProvisionerWithProvider(provisioner, provider).Validate(ctx)).ToNot(Succeed())
			})
		})
	})
})

func ProvisionerWithProvider(provisioner *v1alpha4.Provisioner, provider *v1alpha1.AWS) *v1alpha4.Provisioner {
	raw, err := json.Marshal(provider)
	Expect(err).ToNot(HaveOccurred())
	provisioner.Spec.Constraints.Provider = &runtime.RawExtension{Raw: raw}
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
