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
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/test"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/ptr"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
var fakeEC2API *fake.EC2API
var controller reconcile.Reconciler

var env = test.NewEnvironment(func(e *test.Environment) {
	clientSet := kubernetes.NewForConfigOrDie(e.Config)
	fakeEC2API = &fake.EC2API{}
	cloudProvider := &CloudProvider{
		nodeAPI: &NodeFactory{fakeEC2API},
		launchTemplateProvider: &LaunchTemplateProvider{
			fakeEC2API,
			NewAMIProvider(&fake.SSMAPI{}, clientSet),
			NewSecurityGroupProvider(fakeEC2API),
			cache.New(CacheTTL, CacheCleanupInterval),
		},
		subnetProvider:       NewSubnetProvider(fakeEC2API),
		instanceTypeProvider: NewInstanceTypeProvider(fakeEC2API),
		instanceProvider:     &InstanceProvider{fakeEC2API},
	}
	registry.RegisterOrDie(cloudProvider)
	controller = allocation.NewController(e.Client, clientSet.CoreV1(), cloudProvider)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var ctx context.Context
	var provisioner *v1alpha2.Provisioner

	BeforeEach(func() {
		ctx = context.Background()
		provisioner = &v1alpha2.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
			},
			Spec: v1alpha2.ProvisionerSpec{
				Cluster: v1alpha2.Cluster{
					Name:     ptr.String("test-cluster"),
					Endpoint: "https://test-cluster",
					CABundle: ptr.String("dGVzdC1jbHVzdGVyCg=="),
				},
			},
		}
	})

	AfterEach(func() {
		fakeEC2API.Reset()
		ExpectCleanedUp(env.Client)
		launchTemplateCache.Flush()
	})

	Context("Reconciliation", func() {
		Context("Reserved Labels", func() {
			It("should not schedule a pod with cloud provider reserved labels", func() {
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{AWSLabelPrefix + "unknown": randomdata.SillyName()}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
		})
		Context("Specialized Hardware", func() {
			It("should launch instances for Nvidia GPU resource requests", func() {
				// Setup
				pod1 := test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")},
					},
				})
				// Should pack onto same instance
				pod2 := test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("2")},
					},
				})
				// Should pack onto a separate instance
				pod3 := test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
						Limits:   v1.ResourceList{resources.NvidiaGPU: resource.MustParse("4")},
					},
				})
				ExpectCreated(env.Client, provisioner)
				ExpectCreatedWithStatus(env.Client, pod1, pod2, pod3)
				ExpectReconcileSucceeded(controller, client.ObjectKeyFromObject(provisioner))
				// Assertions
				scheduled1 := ExpectPodExists(env.Client, pod1.GetName(), pod1.GetNamespace())
				scheduled2 := ExpectPodExists(env.Client, pod2.GetName(), pod2.GetNamespace())
				scheduled3 := ExpectPodExists(env.Client, pod3.GetName(), pod3.GetNamespace())
				Expect(scheduled1.Spec.NodeName).To(Equal(scheduled2.Spec.NodeName))
				Expect(scheduled1.Spec.NodeName).ToNot(Equal(scheduled3.Spec.NodeName))
				ExpectNodeExists(env.Client, scheduled1.Spec.NodeName)
				ExpectNodeExists(env.Client, scheduled3.Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
					ContainElements(
						&ec2.FleetLaunchTemplateOverridesRequest{
							InstanceType: aws.String("p3.8xlarge"),
							SubnetId:     aws.String("test-subnet-1"),
						},
					),
				)
				Expect(fakeEC2API.CalledWithCreateFleetInput[1].LaunchTemplateConfigs[0].Overrides).To(
					ContainElements(
						&ec2.FleetLaunchTemplateOverridesRequest{
							InstanceType: aws.String("p3.8xlarge"),
							SubnetId:     aws.String("test-subnet-1"),
						},
					),
				)
			})
			It("should launch instances for AWS Neuron resource requests", func() {
				// Setup
				pod1 := test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
						Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")},
					},
				})
				// Should pack onto same instance
				pod2 := test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
						Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("2")},
					},
				})
				// Should pack onto a separate instance
				pod3 := test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
						Limits:   v1.ResourceList{resources.AWSNeuron: resource.MustParse("4")},
					},
				})
				ExpectCreated(env.Client, provisioner)
				ExpectCreatedWithStatus(env.Client, pod1, pod2, pod3)
				ExpectReconcileSucceeded(controller, client.ObjectKeyFromObject(provisioner))
				// Assertions
				scheduled1 := ExpectPodExists(env.Client, pod1.GetName(), pod1.GetNamespace())
				scheduled2 := ExpectPodExists(env.Client, pod2.GetName(), pod2.GetNamespace())
				scheduled3 := ExpectPodExists(env.Client, pod3.GetName(), pod3.GetNamespace())
				Expect(scheduled1.Spec.NodeName).To(Equal(scheduled2.Spec.NodeName))
				Expect(scheduled1.Spec.NodeName).ToNot(Equal(scheduled3.Spec.NodeName))
				ExpectNodeExists(env.Client, scheduled1.Spec.NodeName)
				ExpectNodeExists(env.Client, scheduled3.Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
					ContainElements(
						&ec2.FleetLaunchTemplateOverridesRequest{
							InstanceType: aws.String("inf1.6xlarge"),
							SubnetId:     aws.String("test-subnet-1"),
						},
					),
				)
				Expect(fakeEC2API.CalledWithCreateFleetInput[1].LaunchTemplateConfigs[0].Overrides).To(
					ContainElements(
						&ec2.FleetLaunchTemplateOverridesRequest{
							InstanceType: aws.String("inf1.6xlarge"),
							SubnetId:     aws.String("test-subnet-1"),
						},
					),
				)
			})
		})
		Context("CapacityType", func() {
			It("should default to on demand", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(aws.StringValue(fakeEC2API.CalledWithCreateFleetInput[0].TargetCapacitySpecification.DefaultTargetCapacityType)).To(Equal(CapacityTypeOnDemand))
				Expect(node.Labels).ToNot(HaveKey(CapacityTypeLabel))
			})
			It("should default to a provisioner's specified capacity type", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{CapacityTypeLabel: CapacityTypeSpot}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(aws.StringValue(fakeEC2API.CalledWithCreateFleetInput[0].TargetCapacitySpecification.DefaultTargetCapacityType)).To(Equal(CapacityTypeSpot))
				Expect(node.Labels).To(HaveKeyWithValue(CapacityTypeLabel, CapacityTypeSpot))
			})
			It("should allow a pod to override the capacity type", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{CapacityTypeLabel: CapacityTypeSpot}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(aws.StringValue(fakeEC2API.CalledWithCreateFleetInput[0].TargetCapacitySpecification.DefaultTargetCapacityType)).To(Equal(CapacityTypeSpot))
				Expect(node.Labels).To(HaveKeyWithValue(CapacityTypeLabel, CapacityTypeSpot))
			})
			It("should not schedule a pod with an invalid capacityType", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{CapacityTypeLabel: "unknown"}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
		})
		Context("LaunchTemplates", func() {
			It("should default to a generated launch template", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(aws.StringValue(launchTemplate.LaunchTemplateId)).To(Equal("test-launch-template-id"))
				Expect(aws.StringValue(launchTemplate.Version)).To(Equal(DefaultLaunchTemplateVersion))
				Expect(node.Labels).ToNot(HaveKey(LaunchTemplateIdLabel))
				Expect(node.Labels).ToNot(HaveKey(LaunchTemplateVersionLabel))
			})
			It("should default to a provisioner's launch template id and version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{
					LaunchTemplateIdLabel:      randomdata.SillyName(),
					LaunchTemplateVersionLabel: randomdata.SillyName(),
				}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(aws.StringValue(launchTemplate.LaunchTemplateId)).To(Equal(provisioner.Spec.Labels[LaunchTemplateIdLabel]))
				Expect(aws.StringValue(launchTemplate.Version)).To(Equal(provisioner.Spec.Labels[LaunchTemplateVersionLabel]))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateIdLabel, provisioner.Spec.Labels[LaunchTemplateIdLabel]))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateVersionLabel, provisioner.Spec.Labels[LaunchTemplateVersionLabel]))
			})
			It("should default to a provisioner's launch template and the default launch template version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{LaunchTemplateIdLabel: randomdata.SillyName()}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(aws.StringValue(launchTemplate.LaunchTemplateId)).To(Equal(provisioner.Spec.Labels[LaunchTemplateIdLabel]))
				Expect(aws.StringValue(launchTemplate.Version)).To(Equal(DefaultLaunchTemplateVersion))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateIdLabel, provisioner.Spec.Labels[LaunchTemplateIdLabel]))
				Expect(node.Labels).ToNot(HaveKey(LaunchTemplateVersionLabel))
			})
			It("should allow a pod to override the launch template id and version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{
					LaunchTemplateIdLabel:      randomdata.SillyName(),
					LaunchTemplateVersionLabel: randomdata.SillyName(),
				}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{
						LaunchTemplateIdLabel:      randomdata.SillyName(),
						LaunchTemplateVersionLabel: randomdata.SillyName(),
					}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(aws.StringValue(launchTemplate.LaunchTemplateId)).To(Equal(pods[0].Spec.NodeSelector[LaunchTemplateIdLabel]))
				Expect(aws.StringValue(launchTemplate.Version)).To(Equal(pods[0].Spec.NodeSelector[LaunchTemplateVersionLabel]))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateIdLabel, pods[0].Spec.NodeSelector[LaunchTemplateIdLabel]))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateVersionLabel, pods[0].Spec.NodeSelector[LaunchTemplateVersionLabel]))
			})
			It("should allow a pod to override the launch template id and use the default launch template version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{LaunchTemplateIdLabel: randomdata.SillyName()}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{LaunchTemplateIdLabel: randomdata.SillyName()}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(aws.StringValue(launchTemplate.LaunchTemplateId)).To(Equal(pods[0].Spec.NodeSelector[LaunchTemplateIdLabel]))
				Expect(aws.StringValue(launchTemplate.Version)).To(Equal(DefaultLaunchTemplateVersion))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateIdLabel, pods[0].Spec.NodeSelector[LaunchTemplateIdLabel]))
				Expect(node.Labels).ToNot(HaveKey(LaunchTemplateVersionLabel))
			})
			It("should allow a pod to override the launch template id and use the provisioner's launch template version", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{
					LaunchTemplateIdLabel:      randomdata.SillyName(),
					LaunchTemplateVersionLabel: randomdata.SillyName(),
				}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{LaunchTemplateIdLabel: randomdata.SillyName()}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				launchTemplate := fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].LaunchTemplateSpecification
				Expect(aws.StringValue(launchTemplate.LaunchTemplateId)).To(Equal(pods[0].Spec.NodeSelector[LaunchTemplateIdLabel]))
				Expect(aws.StringValue(launchTemplate.Version)).To(Equal(provisioner.Spec.Labels[LaunchTemplateVersionLabel]))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateIdLabel, pods[0].Spec.NodeSelector[LaunchTemplateIdLabel]))
				Expect(node.Labels).To(HaveKeyWithValue(LaunchTemplateVersionLabel, provisioner.Spec.Labels[LaunchTemplateVersionLabel]))
			})
		})
		Context("Subnets", func() {
			It("should default to the clusters subnets", func() {
				// Setup
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-1"), InstanceType: aws.String("m5.large")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
				Expect(node.Labels).ToNot(HaveKey(SubnetNameLabel))
				Expect(node.Labels).ToNot(HaveKey(SubnetTagKeyLabel))
			})
			It("should default to a provisioner's specified subnet name", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{SubnetNameLabel: "test-subnet-2"}
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
				))
				Expect(node.Labels).To(HaveKeyWithValue(SubnetNameLabel, provisioner.Spec.Labels[SubnetNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(SubnetTagKeyLabel))
			})
			It("should default to a provisioner's specified subnet tag key", func() {
				provisioner.Spec.Labels = map[string]string{SubnetTagKeyLabel: "TestTag"}
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
				Expect(node.Labels).ToNot(HaveKey(SubnetNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(SubnetTagKeyLabel, provisioner.Spec.Labels[SubnetTagKeyLabel]))
			})
			It("should allow a pod to override the subnet name", func() {
				// Setup
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{SubnetNameLabel: "test-subnet-2"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-2"), InstanceType: aws.String("m5.large")},
				))
				Expect(node.Labels).To(HaveKeyWithValue(SubnetNameLabel, pods[0].Spec.NodeSelector[SubnetNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(SubnetTagKeyLabel))
			})
			It("should allow a pod to override the subnet tags", func() {
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{SubnetTagKeyLabel: "TestTag"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(ConsistOf(
					&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("test-subnet-3"), InstanceType: aws.String("m5.large")},
				))
				Expect(node.Labels).ToNot(HaveKey(SubnetNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(SubnetTagKeyLabel, pods[0].Spec.NodeSelector[SubnetTagKeyLabel]))
			})
			It("should not schedule a pod with an invalid subnet", func() {
				provisioner.Spec.InstanceTypes = []string{"m5.large"} // limit instance type to simplify ConsistOf checks
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningFailed(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{SubnetTagKeyLabel: "Invalid"}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
		})
		Context("Security Groups", func() {
			It("should default to the clusters security groups", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput[0].LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-1"),
					aws.String("test-security-group-2"),
					aws.String("test-security-group-3"),
				))
				Expect(node.Labels).ToNot(HaveKey(SecurityGroupNameLabel))
				Expect(node.Labels).ToNot(HaveKey(SecurityGroupTagKeyLabel))
			})
			It("should default to a provisioner's specified security groups name", func() {
				// Setup
				provisioner.Spec.Labels = map[string]string{SecurityGroupNameLabel: "test-security-group-2"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput[0].LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-2"),
				))
				Expect(node.Labels).To(HaveKeyWithValue(SecurityGroupNameLabel, provisioner.Spec.Labels[SecurityGroupNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(SecurityGroupTagKeyLabel))
			})
			It("should default to a provisioner's specified security groups tag key", func() {
				provisioner.Spec.Labels = map[string]string{SecurityGroupTagKeyLabel: "TestTag"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput[0].LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-3"),
				))
				Expect(node.Labels).ToNot(HaveKey(SecurityGroupNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(SecurityGroupTagKeyLabel, provisioner.Spec.Labels[SecurityGroupTagKeyLabel]))
			})
			It("should allow a pod to override the security groups name", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{SecurityGroupNameLabel: "test-security-group-2"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput[0].LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-2"),
				))
				Expect(node.Labels).To(HaveKeyWithValue(SecurityGroupNameLabel, pods[0].Spec.NodeSelector[SecurityGroupNameLabel]))
				Expect(node.Labels).ToNot(HaveKey(SecurityGroupTagKeyLabel))
			})
			It("should allow a pod to override the security groups tags", func() {
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{SecurityGroupTagKeyLabel: "TestTag"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput).To(HaveLen(1))
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput[0].LaunchTemplateData.SecurityGroupIds).To(ConsistOf(
					aws.String("test-security-group-3"),
				))
				Expect(node.Labels).ToNot(HaveKey(SecurityGroupNameLabel))
				Expect(node.Labels).To(HaveKeyWithValue(SecurityGroupTagKeyLabel, pods[0].Spec.NodeSelector[SecurityGroupTagKeyLabel]))
			})
			It("should not schedule a pod with an invalid security group", func() {
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningFailed(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{SecurityGroupTagKeyLabel: "Invalid"}}),
				)
				// Assertions
				Expect(pods[0].Spec.NodeName).To(BeEmpty())
			})
		})
	})
	Context("Validation", func() {
		Context("Cluster", func() {
			It("should fail if fields are empty", func() {
				for _, cluster := range []v1alpha2.Cluster{
					{Endpoint: "https://test-cluster", CABundle: ptr.String("dGVzdC1jbHVzdGVyCg==")},
					{Name: ptr.String("test-cluster"), CABundle: ptr.String("dGVzdC1jbHVzdGVyCg==")},
					{CABundle: ptr.String("dGVzdC1jbHVzdGVyCg==")},
					{Name: ptr.String("test-cluster")},
				} {
					provisioner.Spec.Cluster = cluster
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				}
			})
		})
		Context("Labels", func() {
			It("should allow unrecognized labels", func() {
				provisioner.Spec.Labels = map[string]string{"foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail if unrecognized aws labels", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/foo": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support launch templates", func() {
				provisioner.Spec.Labels = map[string]string{
					"node.k8s.aws/launch-template-version": randomdata.SillyName(),
					"node.k8s.aws/launch-template-id":      "23",
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should allow launch template id to be specified alone", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/launch-template-id": "23"}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail if only launch template version label present", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/launch-template-version": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support on demand capacity type", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/capacity-type": CapacityTypeOnDemand}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should support spot capacity type", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/capacity-type": CapacityTypeSpot}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail for unrecognized capacity type", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/capacity-type": "foo"}
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
				provisioner.Spec.Architecture = ptr.String("unknown")
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support AMD", func() {
				provisioner.Spec.Architecture = ptr.String(v1alpha2.ArchitectureAmd64)
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should support ARM", func() {
				provisioner.Spec.Architecture = ptr.String(v1alpha2.ArchitectureArm64)
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
		Context("OperatingSystem", func() {
			It("should succeed if unspecified", func() {
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.OperatingSystem = ptr.String("unknown")
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should support linux", func() {
				provisioner.Spec.OperatingSystem = ptr.String(v1alpha2.OperatingSystemLinux)
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
		})
	})
})
