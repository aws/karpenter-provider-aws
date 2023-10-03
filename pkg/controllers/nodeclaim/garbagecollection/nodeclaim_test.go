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
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("NodeClaim/GarbageCollection", func() {
	var instance *ec2.Instance
	var nodeClass *v1beta1.EC2NodeClass
	var providerID string

	BeforeEach(func() {
		instanceID := fake.InstanceID()
		providerID = fake.ProviderID(instanceID)
		nodeClass = test.EC2NodeClass()
		nodePool := coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				},
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
					Key:   aws.String(corev1beta1.NodePoolLabelKey),
					Value: aws.String(nodePool.Name),
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
			InstanceId:   aws.String(instanceID),
			InstanceType: aws.String("m5.large"),
		}
	})
	AfterEach(func() {
		ExpectCleanedUp(ctx, env.Client)
		linkedMachineCache.Flush()
	})

	It("should delete an instance if there is no NodeClaim owner", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).To(HaveOccurred())
		Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
	})
	It("should delete an instance along with the node if there is no NodeClaim owner (to quicken scheduling)", func() {
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
	It("should delete many instances if they all don't have NodeClaim owners", func() {
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
	It("should not delete all instances if they all have NodeClaim owners", func() {
		// Generate 100 instances that have different instanceIDs
		var ids []string
		var nodeClaims []*corev1beta1.NodeClaim
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
					// Launch time was 10m ago
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
			ExpectApplied(ctx, env.Client, nodeClaim)
			nodeClaims = append(nodeClaims, nodeClaim)
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
				Expect(err).ToNot(HaveOccurred())
			}(id)
		}
		wg.Wait()

		for _, nodeClaim := range nodeClaims {
			ExpectExists(ctx, env.Client, nodeClaim)
		}
	})
	It("should not delete an instance if it is within the NodeClaim resolution window (1m)", func() {
		// Launch time just happened
		instance.LaunchTime = aws.Time(time.Now())
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not delete an instance if it was not launched by a NodeClaim", func() {
		// Remove the "karpenter.sh/managed-by" tag (this isn't launched by a machine)
		instance.Tags = lo.Reject(instance.Tags, func(t *ec2.Tag, _ int) bool {
			return aws.StringValue(t.Key) == corev1beta1.ManagedByAnnotationKey
		})

		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).NotTo(HaveOccurred())
	})
	It("should not delete the instance or node if it already has a NodeClaim that matches it", func() {
		// Launch time was 10m ago
		instance.LaunchTime = aws.Time(time.Now().Add(-time.Minute))
		awsEnv.EC2API.Instances.Store(aws.StringValue(instance.InstanceId), instance)

		nodeClaim := coretest.NodeClaim(corev1beta1.NodeClaim{
			Spec: corev1beta1.NodeClaimSpec{
				NodeClassRef: &corev1beta1.NodeClassReference{
					Name: nodeClass.Name,
				},
			},
			Status: corev1beta1.NodeClaimStatus{
				ProviderID: providerID,
			},
		})
		node := coretest.Node(coretest.NodeOptions{
			ProviderID: providerID,
		})
		ExpectApplied(ctx, env.Client, nodeClaim, node)

		ExpectReconcileSucceeded(ctx, garbageCollectionController, client.ObjectKey{})
		_, err := cloudProvider.Get(ctx, providerID)
		Expect(err).ToNot(HaveOccurred())
		ExpectExists(ctx, env.Client, node)
	})
	It("should not delete many instances or nodes if they already have NodeClaim owners that match it", func() {
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
