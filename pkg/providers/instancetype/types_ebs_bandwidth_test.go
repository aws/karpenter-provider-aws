package instancetype_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("EBS bandwidth label emission across EbsOptimizedSupport values", func() {
	var nodeClass *v1.EC2NodeClass
	var nodePool *karpv1.NodePool

	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						ID: "subnet-test1",
					},
				},
			},
			Status: v1.EC2NodeClassStatus{
				Subnets: []v1.Subnet{
					{
						ID:     "subnet-test1",
						Zone:   "test-zone-1a",
						ZoneID: "tstz1-1a",
					},
				},
			},
		})
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						NodeClassRef: &karpv1.NodeClassReference{
							Group: "karpenter.k8s.aws",
							Kind:  "EC2NodeClass",
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodeClass, nodePool)
	})

	AfterEach(func() {
		Expect(env.Client.Delete(ctx, nodePool)).To(Succeed())
		Expect(env.Client.Delete(ctx, nodeClass)).To(Succeed())
	})

	It("should match m8id.8xlarge when ebs-bandwidth affinity is set and EbsOptimizedSupport is Supported", func() {
		// Mock m8id.8xlarge with EbsOptimizedSupport = Supported
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []ec2types.InstanceTypeInfo{
				{
					InstanceType: "m8id.8xlarge",
					VCpuInfo: &ec2types.VCpuInfo{
						DefaultVCpus: aws.Int32(32),
					},
					MemoryInfo: &ec2types.MemoryInfo{
						SizeInMiB: aws.Int64(128 * 1024),
					},
					SupportedUsageClasses: []ec2types.UsageClassType{ec2types.UsageClassTypeOnDemand},
					EbsInfo: &ec2types.EbsInfo{
						EbsOptimizedSupport: ec2types.EbsOptimizedSupportSupported,
						EbsOptimizedInfo: &ec2types.EbsOptimizedInfo{
							MaximumBandwidthInMbps: aws.Int32(10000),
						},
					},
					NetworkInfo: &ec2types.NetworkInfo{
						DefaultNetworkCardIndex:     aws.Int32(0),
						EncryptionInTransitSupported: aws.Bool(true),
						NetworkCards: []ec2types.NetworkCardInfo{
							{
								NetworkCardIndex:         aws.Int32(0),
								MaximumNetworkInterfaces: aws.Int32(8),
							},
						},
						Ipv4AddressesPerInterface: aws.Int32(30),
					},
					ProcessorInfo: &ec2types.ProcessorInfo{
						SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
					},
					PlacementGroupInfo: &ec2types.PlacementGroupInfo{
						SupportedStrategies: []ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
					},
					Hypervisor: ec2types.InstanceTypeHypervisorNitro,
				},
			},
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
				{
					InstanceType: "m8id.8xlarge",
					Location:     aws.String("test-zone-1a"),
				},
			},
		})

		// Trigger an update of instance types
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		// Create a Pod with ebs-bandwidth affinity
		pod := coretest.Pod(coretest.PodOptions{
			Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      "karpenter.k8s.aws/instance-ebs-bandwidth",
					Operator: corev1.NodeSelectorOpGt,
					Values:   []string{"8000"},
				},
			},
		})

		// Check if any instance type matches
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(Succeed())

		// Manual check of the instance type requirements
		m8id, ok := lo.Find(instanceTypes, func(it *corecloudprovider.InstanceType) bool {
			return it.Name == "m8id.8xlarge"
		})
		Expect(ok).To(BeTrue(), "m8id.8xlarge should be found")
		
		req := m8id.Requirements.Get("karpenter.k8s.aws/instance-ebs-bandwidth")
		fmt.Printf("m8id ebs-bandwidth requirement: %v\n", req)
		
		// Now check if the Pod fits
		matchExpressions := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions
		podRequirements := scheduling.NewNodeSelectorRequirements(matchExpressions...)
		Expect(m8id.Requirements.Compatible(podRequirements, scheduling.AllowUndefinedWellKnownLabels)).To(Succeed(), "m8id should be compatible with Pod's ebs-bandwidth requirement")
	})

	It("should NOT fit a 20Gi ephemeral-storage request on c8a.xlarge with default 20Gi root volume", func() {
		// Mock c8a.xlarge
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []ec2types.InstanceTypeInfo{
				{
					InstanceType: "c8a.xlarge",
					VCpuInfo: &ec2types.VCpuInfo{
						DefaultVCpus: aws.Int32(4),
					},
					MemoryInfo: &ec2types.MemoryInfo{
						SizeInMiB: aws.Int64(8 * 1024),
					},
					SupportedUsageClasses: []ec2types.UsageClassType{ec2types.UsageClassTypeOnDemand},
					NetworkInfo: &ec2types.NetworkInfo{
						DefaultNetworkCardIndex:     aws.Int32(0),
						EncryptionInTransitSupported: aws.Bool(true),
						NetworkCards: []ec2types.NetworkCardInfo{
							{
								NetworkCardIndex:         aws.Int32(0),
								MaximumNetworkInterfaces: aws.Int32(4),
							},
						},
						Ipv4AddressesPerInterface: aws.Int32(15),
					},
					ProcessorInfo: &ec2types.ProcessorInfo{
						SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
					},
					PlacementGroupInfo: &ec2types.PlacementGroupInfo{
						SupportedStrategies: []ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
					},
					Hypervisor: ec2types.InstanceTypeHypervisorNitro,
				},
			},
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
				{
					InstanceType: "c8a.xlarge",
					Location:     aws.String("test-zone-1a"),
				},
			},
		})

		// Trigger an update of instance types
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		// Create a Pod with 20Gi ephemeral-storage request
		pod := coretest.Pod(coretest.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceEphemeralStorage: resource.MustParse("20Gi"),
				},
			},
		})

		// Check if any instance type matches
		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(Succeed())

		c8a, ok := lo.Find(instanceTypes, func(it *corecloudprovider.InstanceType) bool {
			return it.Name == "c8a.xlarge"
		})
		Expect(ok).To(BeTrue())

		fmt.Printf("c8a ephemeral-storage capacity: %v\n", c8a.Capacity[corev1.ResourceEphemeralStorage])
		fmt.Printf("c8a ephemeral-storage overhead: %v\n", c8a.Overhead.EvictionThreshold[corev1.ResourceEphemeralStorage])

		// Calculate available storage
		totalOverhead := c8a.Overhead.KubeReserved[corev1.ResourceEphemeralStorage].DeepCopy()
		totalOverhead.Add(c8a.Overhead.SystemReserved[corev1.ResourceEphemeralStorage])
		totalOverhead.Add(c8a.Overhead.EvictionThreshold[corev1.ResourceEphemeralStorage])
		
		available := c8a.Capacity[corev1.ResourceEphemeralStorage].DeepCopy()
		available.Sub(totalOverhead)
		
		fmt.Printf("c8a available ephemeral-storage: %v\n", available)
		fmt.Printf("Pod request: %v\n", pod.Spec.Containers[0].Resources.Requests[corev1.ResourceEphemeralStorage])

		// This should confirm why it doesn't fit
		Expect(available.Cmp(pod.Spec.Containers[0].Resources.Requests[corev1.ResourceEphemeralStorage])).To(BeNumerically("<", 0))
	})
	It("should match c6a.xlarge when ebs-bandwidth affinity is set and EbsOptimizedSupport is Default", func() {
		awsEnv.EC2API.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{
			InstanceTypes: []ec2types.InstanceTypeInfo{
				{
					InstanceType: "c6a.xlarge",
					EbsInfo: &ec2types.EbsInfo{
						EbsOptimizedSupport: ec2types.EbsOptimizedSupportDefault,
						EbsOptimizedInfo: &ec2types.EbsOptimizedInfo{
							MaximumBandwidthInMbps: aws.Int32(1562),
						},
					},
					ProcessorInfo: &ec2types.ProcessorInfo{
						SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
					},
					VCpuInfo: &ec2types.VCpuInfo{
						DefaultVCpus: aws.Int32(4),
					},
					MemoryInfo: &ec2types.MemoryInfo{
						SizeInMiB: aws.Int64(8192),
					},
					SupportedUsageClasses: []ec2types.UsageClassType{ec2types.UsageClassTypeOnDemand},
					NetworkInfo: &ec2types.NetworkInfo{
						DefaultNetworkCardIndex:     aws.Int32(0),
						EncryptionInTransitSupported: aws.Bool(true),
						NetworkCards: []ec2types.NetworkCardInfo{
							{
								NetworkCardIndex:         aws.Int32(0),
								MaximumNetworkInterfaces: aws.Int32(4),
							},
						},
						Ipv4AddressesPerInterface: aws.Int32(15),
					},
					Hypervisor:            ec2types.InstanceTypeHypervisorNitro,
				},
			},
		})
		awsEnv.EC2API.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{
			InstanceTypeOfferings: []ec2types.InstanceTypeOffering{
				{
					InstanceType: "c6a.xlarge",
					Location:     aws.String("test-zone-1a"),
				},
			},
		})
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())

		pod := coretest.Pod(coretest.PodOptions{
			NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      "karpenter.k8s.aws/instance-ebs-bandwidth",
					Operator: corev1.NodeSelectorOpGt,
					Values:   []string{"1000"},
				},
			},
		})

		instanceTypes, err := cloudProvider.GetInstanceTypes(ctx, nodePool)
		Expect(err).To(Succeed())
		
		found := false
		for _, it := range instanceTypes {
			if it.Name == "c6a.xlarge" {
				if err := it.Requirements.Compatible(scheduling.NewNodeSelectorRequirements(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions...)); err == nil {
					found = true
				}
			}
		}
		Expect(found).To(BeTrue(), "c6a should be compatible with Pod's ebs-bandwidth requirement")
	})
})
