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

package fleet

import (
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	cloudprovideraws "github.com/awslabs/karpenter/pkg/cloudprovider/aws"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fleet"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/allocation"
	"github.com/awslabs/karpenter/pkg/test"
	"github.com/awslabs/karpenter/pkg/test/environment"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Provisioner", []Reporter{printer.NewlineReporter{}})
}

var controller *allocation.Controller
var fakeEC2API *fake.EC2API
var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
	clientSet := kubernetes.NewForConfigOrDie(e.Manager.GetConfig())
	fakeEC2API = &fake.EC2API{}
	controller = allocation.NewController(
		e.Manager.GetClient(),
		clientSet.CoreV1(),
		&cloudprovideraws.Factory{FleetFactory: fleet.NewFactory(fakeEC2API, &fake.IAMAPI{}, &fake.SSMAPI{}, e.Manager.GetClient(), clientSet)},
	)
	e.Manager.Register(controller)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var ns *environment.Namespace

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		fakeEC2API.Reset()
		ExpectCleanedUp(ns.Client)
	})

	It("should default to unconstrained instance types", func() {
		// TODO @pgogia
	})
	It("should respect provisioner constrained instance types", func() {
		// TODO @pgogia
	})
	It("should respect pod constrained instance type", func() {
		// TODO @pgogia
	})
	It("should fail conflicting pod constrained instance type", func() {
		// TODO @pgogia
	})
	It("should default to unconstrained zones", func() {
		fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
			{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
			{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
		}}
		// Setup
		prov := &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster"},
			},
		}
		pod := test.PodWith(test.PodOptions{
			Namespace:  ns.Name,
			Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
		})
		ExpectCreatedWithStatus(ns.Client, pod)
		ExpectCreated(ns.Client, prov)
		ExpectEventuallyReconciled(ns.Client, prov)
		// Assertions
		scheduled := &v1.Pod{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
		node := &v1.Node{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
		Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
		Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
			ContainElements(
				&ec2.FleetLaunchTemplateOverridesRequest{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("m5.large"),
					SubnetId:         aws.String("test-subnet-1"),
				},
				&ec2.FleetLaunchTemplateOverridesRequest{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     aws.String("m5.large"),
					SubnetId:         aws.String("test-subnet-2"),
				},
				&ec2.FleetLaunchTemplateOverridesRequest{
					AvailabilityZone: aws.String("test-zone-1c"),
					InstanceType:     aws.String("m5.large"),
					SubnetId:         aws.String("test-subnet-3"),
				},
			))
	})
	It("should respect provisioner constrained zones", func() {
		fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
			{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
			{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
		}}
		// Setup
		prov := &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster"},
				Zones:   []string{"test-zone-1a", "test-zone-1b"},
			},
		}
		pod := test.PodWith(test.PodOptions{
			Namespace:  ns.Name,
			Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
		})
		ExpectCreatedWithStatus(ns.Client, pod)
		ExpectCreated(ns.Client, prov)
		ExpectEventuallyReconciled(ns.Client, prov)
		// Assertions
		scheduled := &v1.Pod{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
		node := &v1.Node{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
		Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
		Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
			ContainElements(
				&ec2.FleetLaunchTemplateOverridesRequest{
					AvailabilityZone: aws.String("test-zone-1a"),
					InstanceType:     aws.String("m5.large"),
					SubnetId:         aws.String("test-subnet-1"),
				},
				&ec2.FleetLaunchTemplateOverridesRequest{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     aws.String("m5.large"),
					SubnetId:         aws.String("test-subnet-2"),
				},
			),
		)
	})
	It("should respect pod constrained zone", func() {
		fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
			{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
			{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
		}}
		// Setup
		prov := &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster"},
				Zones:   []string{"test-zone-1a", "test-zone-1b"},
			},
		}
		pod := test.PodWith(test.PodOptions{
			Namespace:    ns.Name,
			NodeSelector: map[string]string{v1alpha1.ZoneLabelKey: "test-zone-1b"},
			Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
		})
		ExpectCreatedWithStatus(ns.Client, pod)
		ExpectCreated(ns.Client, prov)
		ExpectEventuallyReconciled(ns.Client, prov)
		// Assertions
		scheduled := &v1.Pod{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
		node := &v1.Node{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
		Expect(fakeEC2API.CalledWithCreateFleetInput).To(HaveLen(1))
		Expect(fakeEC2API.CalledWithCreateFleetInput[0].LaunchTemplateConfigs[0].Overrides).To(
			ContainElements(
				&ec2.FleetLaunchTemplateOverridesRequest{
					AvailabilityZone: aws.String("test-zone-1b"),
					InstanceType:     aws.String("m5.large"),
					SubnetId:         aws.String("test-subnet-2"),
				},
			),
		)
	})
	It("should fail conflicting pod constrained zone", func() {
		fakeEC2API.DescribeSubnetsOutput = &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a")},
			{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b")},
			{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c")},
		}}
		// Setup
		prov := &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster"},
				Zones:   []string{"test-zone-1a", "test-zone-1b"},
			},
		}
		pod := test.PodWith(test.PodOptions{
			Namespace:    ns.Name,
			NodeSelector: map[string]string{v1alpha1.ZoneLabelKey: "test-zone-1-c"},
			Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
		})
		ExpectCreatedWithStatus(ns.Client, pod)
		ExpectCreated(ns.Client, prov)
		ExpectEventuallyReconciled(ns.Client, prov)
		// Assertions
		scheduled := &v1.Pod{}
		Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
		Expect(scheduled.Spec.NodeName).To(BeEmpty())
	})
})
