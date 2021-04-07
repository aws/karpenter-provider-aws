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
	"testing"

	"context"

	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	"github.com/patrickmn/go-cache"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/allocation"
	"github.com/awslabs/karpenter/pkg/packing"
	"github.com/awslabs/karpenter/pkg/test"
	webhooksprovisioning "github.com/awslabs/karpenter/pkg/webhooks/provisioning/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "CloudProvider/AWS", []Reporter{printer.NewlineReporter{}})
}

var subnetCache = cache.New(CacheTTL, CacheCleanupInterval)
var launchTemplateCache = cache.New(CacheTTL, CacheCleanupInterval)
var instanceProfileCache = cache.New(CacheTTL, CacheCleanupInterval)
var securityGroupCache = cache.New(CacheTTL, CacheCleanupInterval)
var fakeEC2API *fake.EC2API
var env = test.NewEnvironment(func(e *test.Environment) {
	clientSet := kubernetes.NewForConfigOrDie(e.Manager.GetConfig())
	fakeEC2API = &fake.EC2API{}
	subnetProvider := &SubnetProvider{
		ec2api: fakeEC2API,
		cache:  subnetCache,
	}
	vpcProvider := NewVPCProvider(fakeEC2API, subnetProvider)
	launchTemplateProvider := &LaunchTemplateProvider{
		ec2api: fakeEC2API,
		cache:  launchTemplateCache,
		instanceProfileProvider: &InstanceProfileProvider{
			iamapi:     &fake.IAMAPI{},
			kubeClient: e.Manager.GetClient(),
			cache:      instanceProfileCache,
		},
		securityGroupProvider: &SecurityGroupProvider{
			ec2api: fakeEC2API,
			cache:  securityGroupCache,
		},
		ssm:       &fake.SSMAPI{},
		clientSet: clientSet,
	}
	cloudProviderFactory := &Factory{
		vpcProvider:            vpcProvider,
		nodeFactory:            &NodeFactory{ec2api: fakeEC2API},
		instanceProvider:       &InstanceProvider{ec2api: fakeEC2API, vpc: vpcProvider},
		instanceTypeProvider:   NewInstanceTypeProvider(fakeEC2API),
		packer:                 packing.NewPacker(),
		launchTemplateProvider: launchTemplateProvider,
	}
	e.Manager.RegisterWebhooks(
		&webhooksprovisioning.Validator{CloudProvider: cloudProviderFactory},
		&webhooksprovisioning.Defaulter{},
	).RegisterControllers(
		allocation.NewController(
			e.Manager.GetClient(),
			clientSet.CoreV1(),
			cloudProviderFactory,
		),
	)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha1.Provisioner

	BeforeEach(func() {
		provisioner = &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
			},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{
					Name:     "test-cluster",
					Endpoint: "https://test-cluster",
					CABundle: "dGVzdC1jbHVzdGVyCg==",
				},
			},
		}
	})

	AfterEach(func() {
		fakeEC2API.Reset()
		ExpectCleanedUp(env.Client)
		for _, cache := range []*cache.Cache{
			subnetCache,
			launchTemplateCache,
			instanceProfileCache,
			securityGroupCache,
		} {
			cache.Flush()
		}
	})

	Context("Reconciliation", func() {
		It("should default to a cluster zone", func() {
			fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
				{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
			}}
			// Setup
			pod := test.PodWith(test.PodOptions{
				Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			})
			ExpectCreatedWithStatus(env.Client, pod)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)
			// Assertions
			scheduled := &v1.Pod{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
			node := &v1.Node{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
			Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
			Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
				ContainElements(
					&ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String("m5.large"),
						SubnetId:     aws.String("test-subnet-1"),
					},
					&ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String("m5.large"),
						SubnetId:     aws.String("test-subnet-2"),
					},
					&ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String("m5.large"),
						SubnetId:     aws.String("test-subnet-3"),
					},
				))
		})
		It("should default to a provisioner's zone", func() {
			fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
				{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
			}}
			// Setup
			provisioner.Spec.Zones = []string{"test-zone-1a", "test-zone-1b"}
			pod := test.PodWith(test.PodOptions{
				Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			})
			ExpectCreatedWithStatus(env.Client, pod)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)
			// Assertions
			scheduled := &v1.Pod{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
			node := &v1.Node{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
			Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
			Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
				ContainElements(
					&ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String("m5.large"),
						SubnetId:     aws.String("test-subnet-1"),
					},
					&ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String("m5.large"),
						SubnetId:     aws.String("test-subnet-2"),
					},
				),
			)
		})
		It("should allow pod to override default zone", func() {
			fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
				{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
			}}
			// Setup
			provisioner.Spec.Zones = []string{"test-zone-1a", "test-zone-1b"}
			pod := test.PodWith(test.PodOptions{
				NodeSelector: map[string]string{v1alpha1.ZoneLabelKey: "test-zone-1c"},
				Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			})
			ExpectCreatedWithStatus(env.Client, pod)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)
			// Assertions
			scheduled := &v1.Pod{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
			node := &v1.Node{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
			Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
			Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
				ContainElements(
					&ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String("m5.large"),
						SubnetId:     aws.String("test-subnet-3"),
					},
				),
			)
		})
		It("should launch two instances for pods with different node selectors", func() {
			// Setup
			pod1Options := test.PodOptions{
				NodeSelector: map[string]string{"node.k8s.aws/launch-template-id": randomdata.SillyName()},
				Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}}}
			pod2Options := test.PodOptions{
				NodeSelector: map[string]string{"node.k8s.aws/launch-template-id": randomdata.SillyName()},
				Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}}}
			pod1 := test.PodWith(pod1Options)
			pod2 := test.PodWith(pod2Options)
			ExpectCreatedWithStatus(env.Client, pod1)
			ExpectCreatedWithStatus(env.Client, pod2)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)
			// Assertions
			scheduled1 := &v1.Pod{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: pod1.GetName(), Namespace: pod1.GetNamespace()}, scheduled1)).To(Succeed())
			scheduled2 := &v1.Pod{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: pod2.GetName(), Namespace: pod2.GetNamespace()}, scheduled2)).To(Succeed())
			Expect(scheduled1.Spec.NodeName).NotTo(Equal(scheduled2.Spec.NodeName))
			node1 := &v1.Node{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: scheduled1.Spec.NodeName}, node1)).To(Succeed())
			node2 := &v1.Node{}
			Expect(env.Client.Get(context.Background(), client.ObjectKey{Name: scheduled2.Spec.NodeName}, node2)).To(Succeed())
			Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(2))
		})
	})
	Context("Validation", func() {
		Context("ClusterSpec", func() {
			It("should fail if fields are empty", func() {
				for _, cluster := range []*v1alpha1.ClusterSpec{
					nil,
					{Endpoint: "https://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
					{Name: "test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
					{Name: "test-cluster", Endpoint: "https://test-cluster"},
				} {
					provisioner.Spec.Cluster = cluster
					Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
				}
			})
		})
		Context("Labels", func() {
			It("should fail for restricted labels", func() {
				for _, label := range []string{
					v1alpha1.ArchitectureLabelKey,
					v1alpha1.OperatingSystemLabelKey,
					v1alpha1.ProvisionerNameLabelKey,
					v1alpha1.ProvisionerNamespaceLabelKey,
					v1alpha1.ProvisionerPhaseLabel,
					v1alpha1.ProvisionerTTLKey,
					v1alpha1.ZoneLabelKey,
					v1alpha1.InstanceTypeLabelKey,
				} {
					provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
					Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
				}
			})
			It("should recognize well known labels", func() {
				provisioner.Spec.Labels = map[string]string{
					"node.k8s.aws/launch-template-version": randomdata.SillyName(),
					"node.k8s.aws/launch-template-id":      "23",
				}
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
			It("should fail if only launch template version label present", func() {
				provisioner.Spec.Labels = map[string]string{"node.k8s.aws/launch-template-version": randomdata.SillyName()}
				Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
			})
		})

		Context("Zones", func() {
			It("should succeed if unspecified", func() {
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.Zones = []string{"unknown"}
				Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
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
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
		})
		Context("InstanceTypes", func() {
			It("should succeed if unspecified", func() {
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.InstanceTypes = []string{"unknown"}
				Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
			})
			It("should succeed if supported", func() {
				provisioner.Spec.InstanceTypes = []string{
					"m5.large",
				}
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
		})
		Context("Architecture", func() {
			It("should succeed if unspecified", func() {
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.Architecture = ptr.String("unknown")
				Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
			})
			It("should support AMD", func() {
				provisioner.Spec.Architecture = ptr.String(v1alpha1.ArchitectureAmd64)
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
			It("should support ARM", func() {
				provisioner.Spec.Architecture = ptr.String(v1alpha1.ArchitectureArm64)
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
		})
		Context("OperatingSystem", func() {
			It("should succeed if unspecified", func() {
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
			It("should fail if not supported", func() {
				provisioner.Spec.OperatingSystem = ptr.String("unknown")
				Expect(env.Client.Create(context.Background(), provisioner)).ToNot(Succeed())
			})
			It("should support linux", func() {
				provisioner.Spec.OperatingSystem = ptr.String(v1alpha1.OperatingSystemLinux)
				Expect(env.Client.Create(context.Background(), provisioner)).To(Succeed())
			})
		})
	})
})
