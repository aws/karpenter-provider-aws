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

package garbagecollection_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/nodeclaim/garbagecollection"
	"github.com/aws/karpenter/pkg/controllers/nodeclaim/link"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var awsEnv *test.Environment
var env *coretest.Environment
var garbageCollectionController controller.Controller
var linkedMachineCache *cache.Cache
var cloudProvider *cloudprovider.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine")
}

var _ = BeforeSuite(func() {
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.SubnetProvider)
	linkedMachineCache = cache.New(time.Minute*10, time.Second*10)
	linkController := &link.Controller{
		Cache: linkedMachineCache,
	}
	garbageCollectionController = garbagecollection.NewController(env.Client, cloudProvider, linkController)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
})

var _ = Describe("NodeClaimGarbageCollection", func() {
	var instance *ec2.Instance
	var providerID string

	BeforeEach(func() {
		instanceID := fake.InstanceID()
		providerID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		nodeTemplate := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{})
		provisioner := test.Provisioner(coretest.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{
				APIVersion: "testing/v1alpha1",
				Kind:       "NodeTemplate",
				Name:       nodeTemplate.Name,
			},
		})
		instance = &ec2.Instance{
			State: &ec2.InstanceState{
				Name: aws.String(ec2.InstanceStateNameRunning),
			},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(ctx).ClusterName)),
					Value: aws.String("owned"),
				},
				{
					Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
					Value: aws.String(provisioner.Name),
				},
				{
					Key:   aws.String(v1alpha5.MachineManagedByAnnotationKey),
					Value: aws.String(settings.FromContext(ctx).ClusterName),
				},
			},
			PrivateDnsName: aws.String(fake.PrivateDNSName()),
			Placement: &ec2.Placement{
				AvailabilityZone: aws.String("test-zone-1a"),
			},
			InstanceId:   aws.String(instanceID),
			InstanceType: aws.String("m5.large"),
		}
	})
	AfterEach(func() {
		ExpectCleanedUp(ctx, env.Client)
		linkedMachineCache.Flush()
	})

	It("should delete an instance if there is no machine owner", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
	})
	It("should delete an instance along with the node if there is no machine owner (to quicken scheduling)", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		node := coretest.Node(coretest.NodeOptions{
			ProviderID: providerID,
		})
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())

		ExpectNotFound(ctx, env.Client, node)
	})
	It("should delete many instances if they all don't have machine owners", func() {
		// Generate 100 instances that have different instanceIDs
		var ids []string
		for i := 0; i < 100; i++ {
			instanceID := fake.InstanceID()
			awsEnv.EC2API.Instances.Store(
				instanceID,
				&ec2.Instance{
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
					Tags: []*ec2.Tag{
						{
							Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(ctx).ClusterName)),
							Value: aws.String("owned"),
						},
						{
							Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(v1alpha5.MachineManagedByAnnotationKey),
							Value: aws.String(settings.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("test-zone-1a"),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   aws.String(instanceID),
					InstanceType: aws.String("m5.large"),
				},
			)
			ids = append(ids, instanceID)
		}
		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})

		wg := sync.WaitGroup{}
		for _, id := range ids {
			wg.Add(1)
			go func(id string) {
				defer GinkgoRecover()
				defer wg.Done()

				_, err := cloudProvider.Get(ctx, fmt.Sprintf("aws:///test-zone-1a/%s", id))
				Expect(err).To(HaveOccurred())
				Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
			}(id)
		}
		wg.Wait()
	})
	It("should not delete all instances if they all have machine owners", func() {
		// Generate 100 instances that have different instanceIDs
		var ids []string
		var machines []*v1alpha5.Machine
		for i := 0; i < 100; i++ {
			instanceID := fake.InstanceID()
			awsEnv.EC2API.Instances.Store(
				instanceID,
				&ec2.Instance{
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
					Tags: []*ec2.Tag{
						{
							Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(ctx).ClusterName)),
							Value: aws.String("owned"),
						},
						{
							Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(v1alpha5.MachineManagedByAnnotationKey),
							Value: aws.String(settings.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("test-zone-1a"),
					},
					// Launch time was 10m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   aws.String(instanceID),
					InstanceType: aws.String("m5.large"),
				},
			)
			machine := coretest.Machine(v1alpha5.Machine{
				Status: v1alpha5.MachineStatus{
					ProviderID: fmt.Sprintf("aws:///test-zone-1a/%s", instanceID),
				},
			})
			ExpectApplied(ctx, env.Client, machine)
			machines = append(machines, machine)
			ids = append(ids, instanceID)
		}
		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})

		wg := sync.WaitGroup{}
		for _, id := range ids {
			wg.Add(1)
			go func(id string) {
				defer GinkgoRecover()
				defer wg.Done()

				_, err := cloudProvider.Get(ctx, fmt.Sprintf("aws:///test-zone-1a/%s", id))
				Expect(err).ToNot(HaveOccurred())
			}(id)
		}
		wg.Wait()

		for _, machine := range machines {
			ExpectExists(ctx, env.Client, machine)
		}
	})
	It("should not delete an instance if it is within the machine resolution window (1m)", func() {
		// Launch time just happened
		instance.LaunchTime = aws.Time(time.Now())
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not delete an instance if it was not launched by a machine", func() {
		// Remove the "karpenter.sh/managed-by" tag (this isn't launched by a machine)
		instance.Tags = lo.Reject(instance.Tags, func(t *ec2.Tag, _ int) bool {
			return aws.StringValue(t.Key) == v1alpha5.MachineManagedByAnnotationKey
		})

		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not delete the instance or node if it already has a machine that matches it", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		machine := coretest.Machine(v1alpha5.Machine{
			Status: v1alpha5.MachineStatus{
				ProviderID: providerID,
			},
		})
		node := coretest.Node(coretest.NodeOptions{
			ProviderID: providerID,
		})
		ExpectApplied(ctx, env.Client, machine, node)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).ToNot(HaveOccurred())
		ExpectExists(ctx, env.Client, node)
	})
	It("should not delete an instance if it is linked", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		// Create a machine that is actively linking
		machine := coretest.Machine(v1alpha5.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha5.MachineLinkedAnnotationKey: providerID,
				},
			},
		})
		machine.Status.ProviderID = ""
		ExpectApplied(ctx, env.Client, machine)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not delete an instance if it is recently linked but the machine doesn't exist", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		// Add a provider id to the recently linked cache
		linkedMachineCache.SetDefault(providerID, nil)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).NotTo(HaveOccurred())
	})
})
