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

package drift_test

import (
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
)

var env *aws.Environment
var amdAMI string
var deprecatedAMI string
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestDrift(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Drift")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Drift", Ordered, func() {
	var dep *appsv1.Deployment
	var selector labels.Selector
	var numPods int
	BeforeEach(func() {
		amdAMI = env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", env.K8sVersion()))
		deprecatedAMI = env.GetDeprecatedAMI(amdAMI, "AL2023")
		numPods = 1
		// Add pods with a do-not-disrupt annotation so that we can check node metadata before we disrupt
		dep = coretest.Deployment(coretest.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
					Annotations: map[string]string{
						karpv1.DoNotDisruptAnnotationKey: "true",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})
	It("should disrupt nodes that have drifted due to AMIs", func() {
		oldCustomAMI := env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", env.K8sVersionWithOffset(1)))
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: oldCustomAMI}}

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		env.ExpectCreatedNodeCount("==", 1)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectNodeCount("==", 1)[0]
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMI}}
		env.ExpectCreatedOrUpdated(nodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should disrupt nodes for deprecated AMIs to non-deprecated AMIs", func() {
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: deprecatedAMI}}

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectUpdated(nodeClass)

		By("validating the deprecated status condition has propagated")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).Should(Succeed())
			g.Expect(nodeClass.Status.AMIs[0].Deprecated).To(BeTrue())
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectNodeCount("==", 1)[0]
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMI}, {ID: deprecatedAMI}}
		env.ExpectCreatedOrUpdated(nodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)

		// validate the AMI id matches the non-deprecated AMI
		pod = env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(amdAMI))))

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).Should(Succeed())
			g.Expect(nodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsTrue()).To(BeTrue())
		}).Should(Succeed())
	})
	It("should return drifted if the AMI no longer matches the existing NodeClaims instance type", func() {
		armAMI := env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/arm64/standard/recommended/image_id", env.K8sVersion()))
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: armAMI}}

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		env.ExpectCreatedNodeCount("==", 1)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectNodeCount("==", 1)[0]
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMI}}
		env.ExpectCreatedOrUpdated(nodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should disrupt nodes that have drifted due to securitygroup", func() {
		By("getting the cluster vpc id")
		output, err := env.EKSAPI.DescribeCluster(env.Context, &eks.DescribeClusterInput{Name: awssdk.String(env.ClusterName)})
		Expect(err).To(BeNil())

		By("creating new security group")
		createSecurityGroup := &ec2.CreateSecurityGroupInput{
			GroupName:   awssdk.String("security-group-drift"),
			Description: awssdk.String("End-to-end Drift Test, should delete after drift test is completed"),
			VpcId:       output.Cluster.ResourcesVpcConfig.VpcId,
			TagSpecifications: []ec2types.TagSpecification{
				{
					ResourceType: "security-group",
					Tags: []ec2types.Tag{
						{
							Key:   awssdk.String("karpenter.sh/discovery"),
							Value: awssdk.String(env.ClusterName),
						},
						{
							Key:   awssdk.String(coretest.DiscoveryLabel),
							Value: awssdk.String(env.ClusterName),
						},
						{
							Key:   awssdk.String("creation-date"),
							Value: awssdk.String(time.Now().Format(time.RFC3339)),
						},
					},
				},
			},
		}
		_, _ = env.EC2API.CreateSecurityGroup(env.Context, createSecurityGroup)

		By("looking for security groups")
		var securitygroups []aws.SecurityGroup
		var testSecurityGroup aws.SecurityGroup
		Eventually(func(g Gomega) {
			securitygroups = env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
			testSecurityGroup, _ = lo.Find(securitygroups, func(sg aws.SecurityGroup) bool {
				return awssdk.ToString(sg.GroupName) == "security-group-drift"
			})
			g.Expect(testSecurityGroup).ToNot(BeNil())
		}).Should(Succeed())

		By("creating a new provider with the new securitygroup")
		awsIDs := lo.FilterMap(securitygroups, func(sg aws.SecurityGroup, _ int) (string, bool) {
			if awssdk.ToString(sg.GroupId) != awssdk.ToString(testSecurityGroup.GroupId) {
				return awssdk.ToString(sg.GroupId), true
			}
			return "", false
		})
		sgTerms := []v1.SecurityGroupSelectorTerm{{ID: awssdk.ToString(testSecurityGroup.GroupId)}}
		for _, id := range awsIDs {
			sgTerms = append(sgTerms, v1.SecurityGroupSelectorTerm{ID: id})
		}
		nodeClass.Spec.SecurityGroupSelectorTerms = sgTerms

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		sgTerms = lo.Reject(sgTerms, func(t v1.SecurityGroupSelectorTerm, _ int) bool {
			return t.ID == awssdk.ToString(testSecurityGroup.GroupId)
		})
		nodeClass.Spec.SecurityGroupSelectorTerms = sgTerms
		env.ExpectCreatedOrUpdated(nodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should disrupt nodes that have drifted due to subnets", func() {
		subnets := env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(subnets)).To(BeNumerically(">", 1))

		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{ID: subnets[0].ID}}

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{ID: subnets[1].ID}}
		env.ExpectCreatedOrUpdated(nodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	DescribeTable("EC2NodeClass", func(nodeClassSpec v1.EC2NodeClassSpec) {
		updatedNodeClass := test.EC2NodeClass(v1.EC2NodeClass{Spec: *nodeClass.Spec.DeepCopy()}, v1.EC2NodeClass{Spec: nodeClassSpec})
		updatedNodeClass.ObjectMeta = nodeClass.ObjectMeta

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		env.ExpectCreatedOrUpdated(updatedNodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	},
		Entry("UserData", v1.EC2NodeClassSpec{UserData: awssdk.String("#!/bin/bash\necho \"Hello, AL2023\"")}),
		Entry("Tags", v1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}),
		Entry("MetadataOptions", v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPTokens: awssdk.String("required"), HTTPPutResponseHopLimit: awssdk.Int64(10)}}),
		Entry("BlockDeviceMappings", v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{
			{
				DeviceName: awssdk.String("/dev/xvda"),
				EBS: &v1.BlockDevice{
					VolumeSize: resources.Quantity("20Gi"),
					VolumeType: awssdk.String("gp3"),
					Encrypted:  awssdk.Bool(true),
				},
			}}}),
		Entry("DetailedMonitoring", v1.EC2NodeClassSpec{DetailedMonitoring: awssdk.Bool(true)}),
		Entry("AMIFamily", v1.EC2NodeClassSpec{
			AMISelectorTerms: []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}},
		}),
		Entry("KubeletConfiguration", v1.EC2NodeClassSpec{
			Kubelet: &v1.KubeletConfiguration{
				EvictionSoft:            map[string]string{"memory.available": "5%"},
				EvictionSoftGracePeriod: map[string]metav1.Duration{"memory.available": {Duration: time.Minute}},
			},
		}),
	)
	It("should drift the EC2NodeClass on InstanceProfile", func() {
		// Create a separate test case for this one since we can't use the default NodeClass that's created due to it having
		// a pre-populated role AND we also need to do the instance profile generation within the scope of this test
		instanceProfileName := fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName)
		instanceProfileDriftName := fmt.Sprintf("KarpenterNodeInstanceProfile-Drift-%s", env.ClusterName)
		roleName := fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)

		for _, name := range []string{instanceProfileName, instanceProfileDriftName} {
			env.ExpectInstanceProfileCreated(name, roleName)
			DeferCleanup(func() {
				env.ExpectInstanceProfileDeleted(name, roleName)
			})
		}
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr(instanceProfileName)

		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		nodeClass.Spec.InstanceProfile = lo.ToPtr(instanceProfileDriftName)
		env.ExpectCreatedOrUpdated(nodeClass)

		env.EventuallyExpectDrifted(nodeClaim)

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should drift nodeclaims when spec.role changes", func() {
		// Create initial role and instance profile
		initialRoleName := fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)
		nodeClass.Spec.Role = initialRoleName

		env.ExpectCreated(dep, nodeClass, nodePool)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		firstNodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		firstNode := env.ExpectCreatedNodeCount("==", 1)[0]

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(firstNodeClaim), firstNodeClaim)).To(Succeed())
			g.Expect(nodeClass.Status.InstanceProfile).NotTo(BeEmpty())
			env.EventuallyExpectInstanceProfileExists(nodeClass.Status.InstanceProfile)
			g.Expect(firstNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(nodeClass.Status.InstanceProfile))
		}).Should(Succeed())

		initialInstanceProfile := nodeClass.Status.InstanceProfile

		// Change role
		secondRoleName := fmt.Sprintf("KarpenterNodeRole-%s-%s", env.ClusterName, uuid.New().String()[:8])
		env.EventuallyExpectRoleCreated(secondRoleName)
		DeferCleanup(func() {
			env.ExpectRoleDeleted(secondRoleName)
		})

		nodeClass.Spec.Role = secondRoleName
		env.ExpectCreatedOrUpdated(nodeClass)

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(nodeClass.Status.InstanceProfile).NotTo(BeEmpty())
			g.Expect(nodeClass.Status.InstanceProfile).NotTo(Equal(initialInstanceProfile))
			env.EventuallyExpectInstanceProfileExists(nodeClass.Status.InstanceProfile)
		}).Should(Succeed())

		secondInstanceProfile := nodeClass.Status.InstanceProfile

		env.EventuallyExpectDrifted(firstNodeClaim)
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(firstNodeClaim), firstNodeClaim)).To(Succeed())
			condition := firstNodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted)
			g.Expect(condition).ToNot(BeNil())
			g.Expect(condition.Reason).To(Equal("InstanceProfileDrift"))
		}).Should(Succeed())

		delete(dep.Spec.Template.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(dep)

		env.EventuallyExpectNotFound(firstNodeClaim, firstNode)

		// Verify new nodeclaim uses new instance profile
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		secondNodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		secondNode := env.ExpectCreatedNodeCount("==", 1)[0]
		Expect(secondNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(secondInstanceProfile))

		// Change back to initial role
		nodeClass.Spec.Role = initialRoleName
		env.ExpectCreatedOrUpdated(nodeClass)

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(nodeClass.Status.InstanceProfile).NotTo(BeEmpty())
			g.Expect(nodeClass.Status.InstanceProfile).NotTo(Equal(initialInstanceProfile))
			g.Expect(nodeClass.Status.InstanceProfile).NotTo(Equal(secondInstanceProfile))
			env.EventuallyExpectInstanceProfileExists(nodeClass.Status.InstanceProfile)
		}).Should(Succeed())

		finalInstanceProfile := nodeClass.Status.InstanceProfile

		env.EventuallyExpectDrifted(secondNodeClaim)
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(secondNodeClaim), secondNodeClaim)).To(Succeed())
			condition := secondNodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted)
			g.Expect(condition).ToNot(BeNil())
			g.Expect(condition.Reason).To(Equal("InstanceProfileDrift"))
		}).Should(Succeed())

		env.EventuallyExpectNotFound(secondNodeClaim, secondNode)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		finalNodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		_ = env.ExpectCreatedNodeCount("==", 1)[0]

		// Verify new nodeclaim uses new instance profile
		Expect(finalNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(finalInstanceProfile))
	})
	It("should drift the EC2NodeClass on BlockDeviceMappings volume size update", func() {
		nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
			{
				DeviceName: awssdk.String("/dev/xvda"),
				EBS: &v1.BlockDevice{
					VolumeSize: resources.Quantity("20Gi"),
					VolumeType: awssdk.String("gp3"),
					Encrypted:  awssdk.Bool(true),
				},
			},
		}
		env.ExpectCreated(dep, nodeClass, nodePool)
		pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resources.Quantity("100Gi")
		env.ExpectCreatedOrUpdated(nodeClass)

		By("validating the drifted status condition has propagated")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted)).ToNot(BeNil())
			g.Expect(nodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should update the ec2nodeclass-hash annotation on the ec2nodeclass and nodeclaim when the ec2nodeclass's ec2nodeclass-hash-version annotation does not match the controller hash version", func() {
		env.ExpectCreated(dep, nodeClass, nodePool)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		nodeClass = env.ExpectExists(nodeClass).(*v1.EC2NodeClass)
		expectedHash := nodeClass.Hash()

		By(fmt.Sprintf("expect nodeclass %s and nodeclaim %s to contain %s and %s annotations", nodeClass.Name, nodeClaim.Name, v1.AnnotationEC2NodeClassHash, v1.AnnotationEC2NodeClassHashVersion))
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())

			g.Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
			g.Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
		}).WithTimeout(30 * time.Second).Should(Succeed())

		nodeClass.Annotations = lo.Assign(nodeClass.Annotations, map[string]string{
			v1.AnnotationEC2NodeClassHash:        "test-hash-1",
			v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-1",
		})
		// Updating `nodeClass.Spec.Tags` would normally trigger drift on all nodeclaims using the
		// nodeclass. However, the ec2nodeclass-hash-version does not match the controller hash version, so we will see that
		// none of the nodeclaims will be drifted and all nodeclaims will have an updated `ec2nodeclass-hash` and `ec2nodeclass-hash-version` annotation
		nodeClass.Spec.Tags = lo.Assign(nodeClass.Spec.Tags, map[string]string{
			"test-key": "test-value",
		})
		nodeClaim.Annotations = lo.Assign(nodePool.Annotations, map[string]string{
			v1.AnnotationEC2NodeClassHash:        "test-hash-2",
			v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-2",
		})

		// The nodeclaim will need to be updated first, as the hash controller will only be triggered on changes to the nodeclass
		env.ExpectUpdated(nodeClaim, nodeClass)
		expectedHash = nodeClass.Hash()

		// Expect all nodeclaims not to be drifted and contain an updated `nodepool-hash` and `nodepool-hash-version` annotation
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())

			g.Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
			g.Expect(nodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
			g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
		}).WithTimeout(30 * time.Second).Should(Succeed())
		env.ConsistentlyExpectNodeClaimsNotDrifted(time.Minute, nodeClaim)
	})
	Context("Capacity Reservations", func() {
		var largeCapacityReservationID, xlargeCapacityReservationID string
		BeforeAll(func() {
			largeCapacityReservationID = aws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Large,
				env.ZoneInfo[0].Zone,
				1,
				nil,
				nil,
			)
			xlargeCapacityReservationID = aws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Xlarge,
				env.ZoneInfo[0].Zone,
				1,
				nil,
				nil,
			)
		})
		AfterAll(func() {
			aws.ExpectCapacityReservationsCanceled(env.Context, env.EC2API, largeCapacityReservationID, xlargeCapacityReservationID)
		})
		BeforeEach(func() {
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeReserved},
				},
			}}
		})
		It("should drift nodeclaim when the reservation is no longer selected by the nodeclass", func() {
			nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: largeCapacityReservationID}}
			// Include the do-not-disrupt annotation to prevent replacement NodeClaims from leaking between tests
			pod := coretest.Pod(coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						karpv1.DoNotDisruptAnnotationKey: "true",
					},
				},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)
			nc := env.EventuallyExpectLaunchedNodeClaimCount("==", 1)[0]
			env.EventuallyExpectNodeClaimsReady(nc)
			n := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
			Expect(n.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, string(ec2types.InstanceTypeM5Large)))
			Expect(n.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
			Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, largeCapacityReservationID))

			nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: xlargeCapacityReservationID}}
			env.ExpectUpdated(nodeClass)
			env.EventuallyExpectDrifted(nc)
		})
		It("should drift nodeclaim when the nodeclaim is demoted to on-demand", func() {
			capacityReservationID := aws.ExpectCapacityReservationCreated(
				env.Context,
				env.EC2API,
				ec2types.InstanceTypeM5Large,
				env.ZoneInfo[0].Zone,
				1,
				nil,
				nil,
			)
			DeferCleanup(func() {
				aws.ExpectCapacityReservationsCanceled(env.Context, env.EC2API, capacityReservationID)
			})

			nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: capacityReservationID}}
			// Prevent drift from being executed by marking the pod as do-not-disrupt. Without this, the nodeclaim may be replaced
			// in-between polling intervals for the eventually block.
			pod := coretest.Pod(coretest.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						karpv1.DoNotDisruptAnnotationKey: "true",
					},
				},
			})
			env.ExpectCreated(nodePool, nodeClass, pod)

			nc := env.EventuallyExpectLaunchedNodeClaimCount("==", 1)[0]
			req, ok := lo.Find(nc.Spec.Requirements, func(req karpv1.NodeSelectorRequirementWithMinValues) bool {
				return req.Key == v1.LabelCapacityReservationID
			})
			Expect(ok).To(BeTrue())
			Expect(req.Values).To(ConsistOf(capacityReservationID))
			n := env.EventuallyExpectNodeCount("==", 1)[0]

			aws.ExpectCapacityReservationsCanceled(env.Context, env.EC2API, capacityReservationID)

			// The NodeClaim capacity reservation controller runs once every minute, we'll give a little extra time to avoid
			// a failure from a small delay, but the capacity type label should be updated and the reservation-id label should
			// be removed within a minute of the reservation being canceled.
			Eventually(func(g Gomega) {
				updatedNodeClaim := &karpv1.NodeClaim{}
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nc), updatedNodeClaim)).To(BeNil())
				g.Expect(updatedNodeClaim.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
				g.Expect(updatedNodeClaim.Labels).ToNot(HaveKey(v1.LabelCapacityReservationID))

				updatedNode := &corev1.Node{}
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(n), updatedNode)).To(BeNil())
				g.Expect(updatedNodeClaim.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
				g.Expect(updatedNodeClaim.Labels).ToNot(HaveKey(v1.LabelCapacityReservationID))
			}).WithTimeout(75 * time.Second).Should(Succeed())

			// Since the nodeclaim is only compatible with reserved instances, we should drift the node when it's demoted to on-demand
			env.EventuallyExpectDrifted(nc)
		})
	})
})
