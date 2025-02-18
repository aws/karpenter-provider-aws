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

package instance_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var cloudProvider *cloudprovider.CloudProvider

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	awsEnv.Reset()
})

var _ = Describe("InstanceProvider", func() {
	var nodeClass *v1.EC2NodeClass
	var nodePool *karpv1.NodePool
	var nodeClaim *karpv1.NodeClaim
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})
		nodeClaim = coretest.NodeClaim(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: object.GVK(nodeClass).Group,
					Kind:  object.GVK(nodeClass).Kind,
					Name:  nodeClass.Name,
				},
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
	})
	It("should return an ICE error when all attempted instance types return an ICE error", func() {
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		awsEnv.EC2API.InsufficientCapacityPools.Set([]fake.CapacityPool{
			{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: karpv1.CapacityTypeOnDemand, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
			{CapacityType: karpv1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1a"},
			{CapacityType: karpv1.CapacityTypeSpot, InstanceType: "m5.xlarge", Zone: "test-zone-1b"},
		})
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).ToNot(HaveOccurred())

		// Filter down to a single instance type
		instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })

		// Since all the capacity pools are ICEd. This should return back an ICE error
		instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)
		Expect(corecloudprovider.IsInsufficientCapacityError(err)).To(BeTrue())
		Expect(instance).To(BeNil())
	})
	It("should return all NodePool-owned instances from List", func() {
		ids := sets.New[string]()
		// Provision instances that have the karpenter.sh/nodepool key
		for i := 0; i < 20; i++ {
			instanceID := fake.InstanceID()
			awsEnv.EC2API.Instances.Store(
				instanceID,
				ec2types.Instance{
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameRunning,
					},
					Tags: []ec2types.Tag{
						{
							Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", options.FromContext(ctx).ClusterName)),
							Value: aws.String("owned"),
						},
						{
							Key:   aws.String(karpv1.NodePoolLabelKey),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(v1.LabelNodeClass),
							Value: aws.String("default"),
						},
						{
							Key:   aws.String(v1.EKSClusterNameTagKey),
							Value: aws.String(options.FromContext(ctx).ClusterName),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2types.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   lo.ToPtr(instanceID),
					InstanceType: "m5.large",
				},
			)
			ids.Insert(instanceID)
		}
		// Provision instances that do not have this tag key
		for i := 0; i < 20; i++ {
			instanceID := fake.InstanceID()
			awsEnv.EC2API.Instances.Store(
				instanceID,
				ec2types.Instance{
					State: &ec2types.InstanceState{
						Name: ec2types.InstanceStateNameRunning,
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2types.Placement{
						AvailabilityZone: aws.String(fake.DefaultRegion),
					},
					// Launch time was 1m ago
					LaunchTime:   aws.Time(time.Now().Add(-time.Minute)),
					InstanceId:   lo.ToPtr(instanceID),
					InstanceType: "m5.large",
				},
			)
		}
		instances, err := awsEnv.InstanceProvider.List(ctx)
		Expect(err).To(BeNil())
		Expect(instances).To(HaveLen(20))

		retrievedIDs := sets.New[string](lo.Map(instances, func(i *instance.Instance, _ int) string { return i.ID })...)
		Expect(ids.Equal(retrievedIDs)).To(BeTrue())
	})
		It("should not consider subnet with no available IPs for instance creation", func() {
		// Prepare the context, nodeClass, and nodeClaim as in the other tests
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool, nodeClass)
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)

		// Update the EC2 API mock to include this subnet
    	awsEnv.EC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{
    	    Subnets: []ec2types.Subnet{
    	        {
    	            SubnetId:                aws.String("test-subnet-1"),
    	            AvailabilityZone:        aws.String("test-zone-1a"),
    	            AvailableIpAddressCount: aws.Int32(0), // Exhausted
    	            Tags:                    []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}},
    	        },
    	        {
    	            SubnetId:                aws.String("test-subnet-2"),
    	            AvailabilityZone:        aws.String("test-zone-1b"),
    	            AvailableIpAddressCount: aws.Int32(5), // Has IPs
    	            Tags:                    []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}},
    	        },
    	    },
    	})

    instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
    Expect(err).ToNot(HaveOccurred())

    instanceTypes = lo.Filter(instanceTypes, func(i *corecloudprovider.InstanceType, _ int) bool { return i.Name == "m5.xlarge" })
    instance, err := awsEnv.InstanceProvider.Create(ctx, nodeClass, nodeClaim, nil, instanceTypes)

    // Assert that the instance is created using the subnet with available IPs
    Expect(err).ToNot(HaveOccurred())
    Expect(instance).ToNot(BeNil())
    Expect(instance.SubnetID).To(Equal("test-subnet-2"))
	})
})
