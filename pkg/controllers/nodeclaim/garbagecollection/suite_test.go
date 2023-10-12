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
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"k8s.io/client-go/tools/record"
	. "knative.dev/pkg/logging/testing"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
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

var _ = Describe("Combined/GarbageCollection", func() {
	var nodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
	})
	It("should delete many instances if they all don't have NodeClaim or Machine owners", func() {
		// Generate 100 instances that have different instanceIDs that should have NodeClaims
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
							Key:   aws.String(corev1beta1.NodePoolLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(corev1beta1.ManagedByAnnotationKey),
							Value: aws.String(settings.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   aws.String(instanceID),
					InstanceType: aws.String("m5.large"),
				},
			)
			ids = append(ids, instanceID)
		}
		// Generate 100 instances that have different instanceIDs that should have Machines
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
						AvailabilityZone: aws.String(fake.DefaultRegion),
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

				_, err := cloudProvider.Get(ctx, fake.ProviderID(id))
				Expect(err).To(HaveOccurred())
				Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
			}(id)
		}
		wg.Wait()
	})
	It("should not delete any instance or node if it already has a NodeClaim or Machine owners that matches it", func() {
		// Generate 100 instances that have different instanceIDs that have NodeClaims
		var ids []string
		var nodes []*v1.Node
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
							Key:   aws.String(corev1beta1.NodePoolLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(corev1beta1.ManagedByAnnotationKey),
							Value: aws.String(settings.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   aws.String(instanceID),
					InstanceType: aws.String("m5.large"),
				},
			)
			nodeClaim := coretest.NodeClaim(corev1beta1.NodeClaim{
				Spec: corev1beta1.NodeClaimSpec{
					NodeClassRef: &corev1beta1.NodeClassReference{
						Name: nodeClass.Name,
					},
				},
				Status: corev1beta1.NodeClaimStatus{
					ProviderID: fake.ProviderID(instanceID),
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ProviderID: fake.ProviderID(instanceID),
			})
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ids = append(ids, instanceID)
			nodes = append(nodes, node)
		}
		// Generate 100 instances that have different instanceIDs that have Machines
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
							Key:   aws.String(corev1beta1.NodePoolLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(corev1beta1.ManagedByAnnotationKey),
							Value: aws.String(settings.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   aws.String(instanceID),
					InstanceType: aws.String("m5.large"),
				},
			)
			machine := coretest.Machine(v1alpha5.Machine{
				Status: v1alpha5.MachineStatus{
					ProviderID: fake.ProviderID(instanceID),
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ProviderID: fake.ProviderID(instanceID),
			})
			ExpectApplied(ctx, env.Client, machine, node)
			ids = append(ids, instanceID)
			nodes = append(nodes, node)
		}
		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})

		wg := sync.WaitGroup{}
		for i := range ids {
			wg.Add(1)
			go func(id string, node *v1.Node) {
				defer GinkgoRecover()
				defer wg.Done()

				_, err := cloudProvider.Get(ctx, fake.ProviderID(id))
				Expect(err).ToNot(HaveOccurred())
				ExpectExists(ctx, env.Client, node)
			}(ids[i], nodes[i])
		}
		wg.Wait()
	})
})
