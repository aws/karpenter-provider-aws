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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/binpacking"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/scheduling"
	"github.com/awslabs/karpenter/pkg/test"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	"github.com/awslabs/karpenter/pkg/utils/options"
	"github.com/awslabs/karpenter/pkg/utils/parallel"
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
var opts options.Options

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	opts = options.Options{
		ClusterName:     "test-cluster",
		ClusterEndpoint: "https://test-cluster",
	}
	ctx = options.Inject(ctx, opts)
	launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
	fakeEC2API = &fake.EC2API{}
	subnetProvider := NewSubnetProvider(fakeEC2API)
	instanceTypeProvider := NewInstanceTypeProvider(fakeEC2API, subnetProvider)
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		clientSet := kubernetes.NewForConfigOrDie(e.Config)
		cloudProvider := &CloudProvider{
			subnetProvider:       subnetProvider,
			instanceTypeProvider: instanceTypeProvider,
			instanceProvider: &InstanceProvider{
				fakeEC2API, instanceTypeProvider, subnetProvider, &LaunchTemplateProvider{
					fakeEC2API,
					NewAMIProvider(&fake.SSMAPI{}, clientSet),
					NewSecurityGroupProvider(fakeEC2API),
					launchTemplateCache,
				},
			},
			creationQueue: parallel.NewWorkQueue(CreationQPS, CreationBurst),
		}
		registry.RegisterOrDie(ctx, cloudProvider)
		controller = &allocation.Controller{
			Batcher:   allocation.NewBatcher(1*time.Millisecond, 1*time.Millisecond),
			Filter:    &allocation.Filter{KubeClient: e.Client},
			Scheduler: scheduling.NewScheduler(e.Client, cloudProvider),
			Launcher: &allocation.Launcher{
				Packer:        &binpacking.Packer{},
				KubeClient:    e.Client,
				CoreV1Client:  clientSet.CoreV1(),
				CloudProvider: cloudProvider,
			},
			KubeClient:    e.Client,
			CloudProvider: cloudProvider,
		}
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
			It("should launch spot capacity if flexible to both spot and on demand", func() {
				// Setup
				provisioner.Spec.Requirements = v1alpha5.Requirements{{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}}}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeSpot}}),
				)
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
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
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ContainElements(
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
