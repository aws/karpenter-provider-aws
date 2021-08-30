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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	v1alpha1 "github.com/awslabs/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/binpacking"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/scheduling"
	"github.com/awslabs/karpenter/pkg/test"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
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
			launchTemplateProvider: &LaunchTemplateProvider{
				fakeEC2API,
				NewAMIProvider(&fake.SSMAPI{}, clientSet),
				NewSecurityGroupProvider(fakeEC2API),
				launchTemplateCache,
			},
			subnetProvider:       NewSubnetProvider(fakeEC2API),
			instanceTypeProvider: instanceTypeProvider,
			instanceProvider:     &InstanceProvider{fakeEC2API, instanceTypeProvider},
			creationQueue:        parallel.NewWorkQueue(CreationQPS, CreationBurst),
		}
		registry.RegisterOrDie(cloudProvider)
		controller = &allocation.Controller{
			Filter:        &allocation.Filter{KubeClient: e.Client},
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
	var provisioner *v1alpha3.Provisioner

	BeforeEach(func() {
		awsextensions, err := json.Marshal(&v1alpha1.AWS{
			Cluster: v1alpha1.Cluster{
				Name:     "test-cluster",
				Endpoint: "https://test-cluster",
			},
		})
		Expect(err).ToNot(HaveOccurred())
		provisioner = &v1alpha3.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha3.DefaultProvisioner.Name,
			},
			Spec: v1alpha3.ProvisionerSpec{
				Constraints: v1alpha3.Constraints{
					Provider: &runtime.RawExtension{
						Raw: awsextensions,
					},
				},
			},
		}
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
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(2))
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
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(2))
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
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.CapacityTypeLabel))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeOnDemand))
			})
			It("should default to a provisioner's specified capacity type", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{v1alpha1.CapacityTypeLabel: v1alpha1.CapacityTypeSpot}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.CapacityTypeLabel, v1alpha1.CapacityTypeSpot))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeSpot))
			})
			It("should allow a pod to override the capacity type", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.CapacityTypeLabel: v1alpha1.CapacityTypeSpot}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.CapacityTypeLabel, v1alpha1.CapacityTypeSpot))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(*input.TargetCapacitySpecification.DefaultTargetCapacityType).To(Equal(v1alpha1.CapacityTypeSpot))
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
			It("should default to a generated launch template", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.LaunchTemplateNameLabel))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.Version).To(Equal(v1alpha1.DefaultLaunchTemplateVersion))
			})
			It("should default to a provisioner's launch template id and version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{
					v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName(),
				}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LaunchTemplateNameLabel, provisioner.Spec.Labels[v1alpha1.LaunchTemplateNameLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal(provisioner.Spec.Labels[v1alpha1.LaunchTemplateNameLabel]))
			})
			It("should default to a provisioner's launch template and the default launch template version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName()}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LaunchTemplateNameLabel, provisioner.Spec.Labels[v1alpha1.LaunchTemplateNameLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal(provisioner.Spec.Labels[v1alpha1.LaunchTemplateNameLabel]))
				Expect(*launchTemplate.Version).To(Equal(v1alpha1.DefaultLaunchTemplateVersion))
			})
			It("should allow a pod to override the launch template id and version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{
					v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName(),
				}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{
						v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName(),
					}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LaunchTemplateNameLabel, pods[0].Spec.NodeSelector[v1alpha1.LaunchTemplateNameLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal(pods[0].Spec.NodeSelector[v1alpha1.LaunchTemplateNameLabel]))
			})
			It("should allow a pod to override the launch template name and use the default launch template version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName()}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName()}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LaunchTemplateNameLabel, pods[0].Spec.NodeSelector[v1alpha1.LaunchTemplateNameLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal(pods[0].Spec.NodeSelector[v1alpha1.LaunchTemplateNameLabel]))
				Expect(*launchTemplate.Version).To(Equal(v1alpha1.DefaultLaunchTemplateVersion))
			})
			It("should allow a pod to override the launch template name and use the provisioner's launch template version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{
					v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName(),
				}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.LaunchTemplateNameLabel: randomdata.SillyName()}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LaunchTemplateNameLabel, pods[0].Spec.NodeSelector[v1alpha1.LaunchTemplateNameLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(*launchTemplate.LaunchTemplateName).To(Equal(pods[0].Spec.NodeSelector[v1alpha1.LaunchTemplateNameLabel]))
			})
		})
		Context("Subnets", func() {
			It("should default to the cluster's subnets", func() {
				// Setup
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SubnetNameLabel))
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SubnetTagKeyLabel))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-1"), InstanceType: aws.String("m5.large")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
			})
			It("should default to a provisioner's specified subnet name", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{v1alpha1.SubnetNameLabel: "test-subnet-2"}
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SubnetNameLabel, provisioner.Spec.Labels[v1alpha1.SubnetNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SubnetTagKeyLabel))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
				))
			})
			It("should default to a provisioner's specified subnet tag key", func() {
				provisioner.Spec.Labels = map[string]string{v1alpha1.SubnetTagKeyLabel: "TestTag"}
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SubnetNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SubnetTagKeyLabel, provisioner.Spec.Labels[v1alpha1.SubnetTagKeyLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
			})
			It("should allow a pod to override the subnet name", func() {
				// Setup
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.SubnetNameLabel: "test-subnet-2"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SubnetTagKeyLabel))
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SubnetNameLabel, pods[0].Spec.NodeSelector[v1alpha1.SubnetNameLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
				))
			})
			It("should allow a pod to override the subnet tags", func() {
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.SubnetTagKeyLabel: "TestTag"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SubnetNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SubnetTagKeyLabel, pods[0].Spec.NodeSelector[v1alpha1.SubnetTagKeyLabel]))
				Expect(fakeEC2API.CalledWithCreateFleetInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateFleetInput.Pop().(*ec2.CreateFleetInput)
				Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
				Expect(input.LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
			})
			It("should not schedule a pod with an invalid subnet", func() {
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.SubnetTagKeyLabel: "Invalid"}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
		})
		Context("Security Groups", func() {
			It("should default to the clusters security groups", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SecurityGroupNameLabel))
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SecurityGroupTagKeyLabel))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-1"),
					aws.String("test-security-group-2"),
					aws.String("test-security-group-3"),
				))
			})
			It("should default to a provisioner's specified security groups name", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{v1alpha1.SecurityGroupNameLabel: "test-security-group-2"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SecurityGroupNameLabel, provisioner.Spec.Labels[v1alpha1.SecurityGroupNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SecurityGroupTagKeyLabel))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-2"),
				))
			})
			It("should default to a provisioner's specified security groups tag key", func() {
				provisioner.Spec.Labels = map[string]string{v1alpha1.SecurityGroupTagKeyLabel: "TestTag"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SecurityGroupNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SecurityGroupTagKeyLabel, provisioner.Spec.Labels[v1alpha1.SecurityGroupTagKeyLabel]))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-3"),
				))
			})
			It("should allow a pod to override the security groups name", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.SecurityGroupNameLabel: "test-security-group-2"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SecurityGroupNameLabel, pods[0].Spec.NodeSelector[v1alpha1.SecurityGroupNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SecurityGroupTagKeyLabel))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-2"),
				))
			})
			It("should allow a pod to override the security groups tags", func() {
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.SecurityGroupTagKeyLabel: "TestTag"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).ToNot(HaveKey(v1alpha1.SecurityGroupNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.SecurityGroupTagKeyLabel, pods[0].Spec.NodeSelector[v1alpha1.SecurityGroupTagKeyLabel]))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Cardinality()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().(*ec2.CreateLaunchTemplateInput)
				Expect(input.LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-3"),
				))
			})
			It("should not schedule a pod with an invalid security group", func() {
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha1.SecurityGroupTagKeyLabel: "Invalid"}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
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
					provisioner.Spec.Constraints.Provider.Raw, _ = json.Marshal(&v1alpha1.AWS{Cluster: cluster})
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
					provisioner.Spec.Constraints.Provider.Raw, _ = json.Marshal(&v1alpha1.AWS{Cluster: v1alpha1.Cluster{Name: "test-cluster", Endpoint: endpoint}})
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("Labels", func() {
			It("should allow unrecognized labels", func() {
				provisioner.Spec.Labels = map[string]string{"foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should not allow aws labels reserved for pod node selectors", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})

		Context("Zones", func() {
			It("should succeed if unspecified", func() {
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
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
			It("should succeed if unspecified", func() {
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
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
			It("should succeed if unspecified", func() {
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.Architectures = []string{"unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support AMD", func() {
				provisioner.Spec.Architectures = []string{v1alpha3.ArchitectureAmd64}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should support ARM", func() {
				provisioner.Spec.Architectures = []string{v1alpha3.ArchitectureArm64}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
		Context("OperatingSystem", func() {
			It("should succeed if unspecified", func() {
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.OperatingSystems = []string{"unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support linux", func() {
				provisioner.Spec.OperatingSystems = []string{v1alpha3.OperatingSystemLinux}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
	})
})
