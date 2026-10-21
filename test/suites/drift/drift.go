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

package drift

import (
	"fmt"
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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/utils/resources"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"
)

var Env *aws.Environment
var amdAMI string
var deprecatedAMI string
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	return Describe("Drift", Ordered, func() {
		var dep *appsv1.Deployment
		var selector labels.Selector
		var numPods int
		BeforeEach(func() {
			amdAMI = Env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", Env.K8sVersion()))
			deprecatedAMI = Env.GetDeprecatedAMI(amdAMI, "AL2023")
			numPods = 1
			// Add pods with a do-not-disrupt annotation so that we can check node metadata before we disrupt
			dep = coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods), //nolint:gosec
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
			oldCustomAMI := Env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", Env.K8sVersionWithOffset(1)))
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: oldCustomAMI}}

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			Env.ExpectCreatedNodeCount("==", 1)

			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.EventuallyExpectNodeCount("==", 1)[0]
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMI}}
			Env.ExpectCreatedOrUpdated(NodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, nodeClaim, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
		})
		It("should disrupt nodes for deprecated AMIs to non-deprecated AMIs", func() {
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: deprecatedAMI}}

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			Env.ExpectCreatedNodeCount("==", 1)
			Env.ExpectUpdated(NodeClass)

			By("validating the deprecated status condition has propagated")
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).Should(Succeed())
				g.Expect(NodeClass.Status.AMIs[0].Deprecated).To(BeTrue())
				g.Expect(NodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsTrue()).To(BeTrue())
			}).Should(Succeed())

			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.EventuallyExpectNodeCount("==", 1)[0]
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMI}, {ID: deprecatedAMI}}
			Env.ExpectCreatedOrUpdated(NodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, nodeClaim, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)

			// validate the AMI id matches the non-deprecated AMI
			pod = Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(amdAMI))))

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).Should(Succeed())
				g.Expect(NodeClass.StatusConditions().Get(v1.ConditionTypeAMIsReady).IsTrue()).To(BeTrue())
			}).Should(Succeed())
		})
		It("should return drifted if the AMI no longer matches the existing NodeClaims instance type", func() {
			armAMI := Env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/arm64/standard/recommended/image_id", Env.K8sVersion()))
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: armAMI}}

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			Env.ExpectCreatedNodeCount("==", 1)

			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.EventuallyExpectNodeCount("==", 1)[0]
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: amdAMI}}
			Env.ExpectCreatedOrUpdated(NodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, nodeClaim, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
		})
		It("should disrupt nodes that have drifted due to securitygroup", func() {
			By("getting the cluster vpc id")
			output, err := Env.EKSAPI.DescribeCluster(Env.Context, &eks.DescribeClusterInput{Name: awssdk.String(Env.ClusterName)})
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
								Value: awssdk.String(Env.ClusterName),
							},
							{
								Key:   awssdk.String(coretest.DiscoveryLabel),
								Value: awssdk.String(Env.ClusterName),
							},
							{
								Key:   awssdk.String("creation-date"),
								Value: awssdk.String(time.Now().Format(time.RFC3339)),
							},
						},
					},
				},
			}
			_, _ = Env.EC2API.CreateSecurityGroup(Env.Context, createSecurityGroup)

			By("looking for security groups")
			var securitygroups []aws.SecurityGroup
			var testSecurityGroup aws.SecurityGroup
			Eventually(func(g Gomega) {
				securitygroups = Env.GetSecurityGroups(Env.DiscoveryTags())
				testSecurityGroup, _ = lo.Find(securitygroups, func(sg aws.SecurityGroup) bool {
					return awssdk.ToString(sg.GroupName) == "security-group-drift"
				})
				g.Expect(testSecurityGroup).ToNot(BeNil())
			}).Should(Succeed())

			// Clean up the temporary security group we created for these tests
			DeferCleanup(func() {
				Env.ExpectSecurityGroupDeleted(testSecurityGroup.GroupId)
			})

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
			NodeClass.Spec.SecurityGroupSelectorTerms = sgTerms

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			sgTerms = lo.Reject(sgTerms, func(t v1.SecurityGroupSelectorTerm, _ int) bool {
				return t.ID == awssdk.ToString(testSecurityGroup.GroupId)
			})
			NodeClass.Spec.SecurityGroupSelectorTerms = sgTerms
			Env.ExpectCreatedOrUpdated(NodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, nodeClaim, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
		})
		It("should disrupt nodes that have drifted due to subnets", func() {
			subnets := Env.DiscoveredSubnetInfo()
			Expect(len(subnets)).To(BeNumerically(">", 1))

			NodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{ID: subnets[0].ID}}

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			NodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{ID: subnets[1].ID}}
			Env.ExpectCreatedOrUpdated(NodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
		})
		DescribeTable("EC2NodeClass", func(nodeClassSpec v1.EC2NodeClassSpec) {
			updatedNodeClass := test.EC2NodeClass(v1.EC2NodeClass{Spec: *NodeClass.Spec.DeepCopy()}, v1.EC2NodeClass{Spec: nodeClassSpec})
			updatedNodeClass.ObjectMeta = NodeClass.ObjectMeta

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			Env.ExpectCreatedOrUpdated(updatedNodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
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
				Kubelet: test.MustMakeKubeletConfiguration(map[string]interface{}{
					"evictionSoft":            map[string]string{"memory.available": "5%"},
					"evictionSoftGracePeriod": map[string]string{"memory.available": "1m0s"},
				}),
			}),
			// Drift keys off the NodeClass spec, so adding a maxPods CEL expression must drift the node even
			// though the expression is only resolved to a concrete value later. Requires the NodeClassCEL gate,
			// which the e2e install enables via test/hack/e2e_scripts/install_karpenter.sh.
			Entry("KubeletConfiguration with a CEL expression", v1.EC2NodeClassSpec{
				Kubelet: test.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": "min(110, vcpus * 20)",
				}),
			}),
			Entry("NetworkInterfaces", v1.EC2NodeClassSpec{
				NetworkInterfaces: []*v1.NetworkInterface{
					{NetworkCardIndex: 0, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface},
				},
			}),
		)
		It("should drift the EC2NodeClass on InstanceProfile", func() {
			// Create a separate test case for this one since we can't use the default NodeClass that's created due to it having
			// a pre-populated role AND we also need to do the instance profile generation within the scope of this test
			instanceProfileName := fmt.Sprintf("KarpenterNodeInstanceProfile-%s", Env.ClusterName)
			instanceProfileDriftName := fmt.Sprintf("KarpenterNodeInstanceProfile-Drift-%s", Env.ClusterName)
			roleName := fmt.Sprintf("KarpenterNodeRole-%s", Env.ClusterName)

			for _, name := range []string{instanceProfileName, instanceProfileDriftName} {
				Env.ExpectInstanceProfileCreated(name, roleName)
				DeferCleanup(func() {
					Env.ExpectInstanceProfileDeleted(name, roleName)
				})
			}
			NodeClass.Spec.Role = ""
			NodeClass.Spec.InstanceProfile = lo.ToPtr(instanceProfileName)

			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			NodeClass.Spec.InstanceProfile = lo.ToPtr(instanceProfileDriftName)
			Env.ExpectCreatedOrUpdated(NodeClass)

			Env.EventuallyExpectDrifted(nodeClaim)

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
		})
		It("should drift nodeclaims when spec.role changes", Label("needs:instance-profile-annotation"), func() {
			// Create initial role and instance profile
			initialRoleName := NodeClass.Spec.Role

			Env.ExpectCreated(dep, NodeClass, NodePool)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			firstNodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			firstNode := Env.ExpectCreatedNodeCount("==", 1)[0]

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).To(Succeed())
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(firstNodeClaim), firstNodeClaim)).To(Succeed())
				g.Expect(NodeClass.Status.InstanceProfile).NotTo(BeEmpty())
				Env.EventuallyExpectInstanceProfileExists(NodeClass.Status.InstanceProfile)
				g.Expect(firstNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(NodeClass.Status.InstanceProfile))
			}).Should(Succeed())

			initialInstanceProfile := NodeClass.Status.InstanceProfile

			// Change role
			secondRoleName := fmt.Sprintf("KarpenterNodeRole-%s-%s", Env.ClusterName, uuid.New().String()[:8])
			Env.EventuallyExpectNodeRoleCreated(secondRoleName)
			DeferCleanup(func() {
				Env.ExpectNodeRoleDeleted(secondRoleName)
			})

			NodeClass.Spec.Role = secondRoleName
			Env.ExpectCreatedOrUpdated(NodeClass)

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).To(Succeed())
				g.Expect(NodeClass.Status.InstanceProfile).NotTo(BeEmpty())
				g.Expect(NodeClass.Status.InstanceProfile).NotTo(Equal(initialInstanceProfile))
				Env.EventuallyExpectInstanceProfileExists(NodeClass.Status.InstanceProfile)
			}).Should(Succeed())

			secondInstanceProfile := NodeClass.Status.InstanceProfile

			Env.EventuallyExpectDrifted(firstNodeClaim)
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(firstNodeClaim), firstNodeClaim)).To(Succeed())
				condition := firstNodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted)
				g.Expect(condition).ToNot(BeNil())
				g.Expect(condition.Reason).To(Equal("NodeClassDrift"))
			}).Should(Succeed())

			delete(dep.Spec.Template.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(dep)

			Env.EventuallyExpectNotFound(firstNodeClaim, firstNode)

			// Verify new nodeclaim uses new instance profile
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			secondNodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			secondNode := Env.ExpectCreatedNodeCount("==", 1)[0]
			Expect(secondNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(secondInstanceProfile))

			// Change back to initial role
			NodeClass.Spec.Role = initialRoleName
			Env.ExpectCreatedOrUpdated(NodeClass)

			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).To(Succeed())
				g.Expect(NodeClass.Status.InstanceProfile).NotTo(BeEmpty())
				g.Expect(NodeClass.Status.InstanceProfile).NotTo(Equal(secondInstanceProfile))
				Env.EventuallyExpectInstanceProfileExists(NodeClass.Status.InstanceProfile)
			}).Should(Succeed())

			finalInstanceProfile := NodeClass.Status.InstanceProfile

			Env.EventuallyExpectDrifted(secondNodeClaim)
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(secondNodeClaim), secondNodeClaim)).To(Succeed())
				condition := secondNodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted)
				g.Expect(condition).ToNot(BeNil())
				g.Expect(condition.Reason).To(Equal("NodeClassDrift"))
			}).Should(Succeed())

			Env.EventuallyExpectNotFound(secondNodeClaim, secondNode)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			finalNodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			finalNode := Env.ExpectCreatedNodeCount("==", 1)[0]

			// Verify new nodeclaim uses new instance profile
			Expect(finalNodeClaim.Annotations[v1.AnnotationInstanceProfile]).To(Equal(finalInstanceProfile))

			Expect(Env.ExpectTestingFinalizerRemoved(finalNode)).To(Succeed())
			Expect(Env.ExpectTestingFinalizerRemoved(finalNodeClaim)).To(Succeed())

			Env.ExpectDeleted(NodePool)
			Env.EventuallyExpectNotFound(finalNodeClaim, finalNode)
			Env.ExpectDeleted(NodeClass)
			Env.EventuallyExpectInstanceProfilesNotFound(initialInstanceProfile, secondInstanceProfile, finalInstanceProfile)
		})
		It("should drift the EC2NodeClass on BlockDeviceMappings volume size update", func() {
			NodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: awssdk.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize: resources.Quantity("20Gi"),
						VolumeType: awssdk.String("gp3"),
						Encrypted:  awssdk.Bool(true),
					},
				},
			}
			Env.ExpectCreated(dep, NodeClass, NodePool)
			pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			NodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resources.Quantity("100Gi")
			Env.ExpectCreatedOrUpdated(NodeClass)

			By("validating the drifted status condition has propagated")
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
				g.Expect(nodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted)).ToNot(BeNil())
				g.Expect(nodeClaim.StatusConditions().Get(karpv1.ConditionTypeDrifted).IsTrue()).To(BeTrue())
			}).Should(Succeed())

			delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
			Env.ExpectUpdated(pod)
			Env.EventuallyExpectNotFound(pod, node)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
		})
		It("should update the ec2nodeclass-hash annotation on the ec2nodeclass and nodeclaim when the ec2nodeclass's ec2nodeclass-hash-version annotation does not match the controller hash version", Label("needs:ec2nodeclass-hash"), func() {
			Env.ExpectCreated(dep, NodeClass, NodePool)
			Env.EventuallyExpectHealthyPodCount(selector, numPods)
			nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
			NodeClass = Env.ExpectExists(NodeClass).(*v1.EC2NodeClass)
			expectedHash := NodeClass.Hash(lo.ToPtr(Env.ExpectCABundle()))

			By(fmt.Sprintf("expect nodeclass %s and nodeclaim %s to contain %s and %s annotations", NodeClass.Name, nodeClaim.Name, v1.AnnotationEC2NodeClassHash, v1.AnnotationEC2NodeClassHashVersion))
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).To(Succeed())
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())

				g.Expect(NodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
				g.Expect(NodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
				g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
				g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
			}).WithTimeout(30 * time.Second).Should(Succeed())

			NodeClass.Annotations = lo.Assign(NodeClass.Annotations, map[string]string{
				v1.AnnotationEC2NodeClassHash:        "test-hash-1",
				v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-1",
			})
			// Updating `nodeClass.Spec.Tags` would normally trigger drift on all nodeclaims using the
			// nodeclass. However, the ec2nodeclass-hash-version does not match the controller hash version, so we will see that
			// none of the nodeclaims will be drifted and all nodeclaims will have an updated `ec2nodeclass-hash` and `ec2nodeclass-hash-version` annotation
			NodeClass.Spec.Tags = lo.Assign(NodeClass.Spec.Tags, map[string]string{
				"test-key": "test-value",
			})
			nodeClaim.Annotations = lo.Assign(NodePool.Annotations, map[string]string{
				v1.AnnotationEC2NodeClassHash:        "test-hash-2",
				v1.AnnotationEC2NodeClassHashVersion: "test-hash-version-2",
			})

			// The nodeclaim will need to be updated first, as the hash controller will only be triggered on changes to the nodeclass
			Env.ExpectUpdated(nodeClaim, NodeClass)
			expectedHash = NodeClass.Hash(lo.ToPtr(Env.ExpectCABundle()))

			// Expect all nodeclaims not to be drifted and contain an updated `nodepool-hash` and `nodepool-hash-version` annotation
			Eventually(func(g Gomega) {
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)).To(Succeed())
				g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())

				g.Expect(NodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
				g.Expect(NodeClass.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
				g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHash, expectedHash))
				g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationEC2NodeClassHashVersion, v1.EC2NodeClassHashVersion))
			}).WithTimeout(30 * time.Second).Should(Succeed())
			Env.ConsistentlyExpectNodeClaimsNotDrifted(time.Minute, nodeClaim)
		})
		Context("Capacity Reservations", Label("needs:capacity-reservation"), func() {
			var largeCapacityReservation, xlargeCapacityReservation aws.CapacityReservation
			BeforeAll(func() {
				largeCapacityReservation, xlargeCapacityReservation = Env.ExpectCapacityReservationPairCreated(1, 1)
			})
			AfterAll(func() {
				aws.ExpectCapacityReservationsCanceled(Env.Context, Env.EC2API, largeCapacityReservation.ID, xlargeCapacityReservation.ID)
			})
			BeforeEach(func() {
				NodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeReserved},
				}}
			})
			It("should drift nodeclaim when the reservation is no longer selected by the nodeclass", func() {
				NodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: largeCapacityReservation.ID}}
				// Include the do-not-disrupt annotation to prevent replacement NodeClaims from leaking between tests
				pod := coretest.Pod(coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							karpv1.DoNotDisruptAnnotationKey: "true",
						},
					},
				})
				Env.ExpectCreated(NodePool, NodeClass, pod)
				nc := Env.EventuallyExpectLaunchedNodeClaimCount("==", 1)[0]
				Env.EventuallyExpectNodeClaimsReady(nc)
				n := Env.EventuallyExpectCreatedNodeCount("==", 1)[0]
				Expect(n.Labels).To(HaveKeyWithValue(corev1.LabelInstanceTypeStable, largeCapacityReservation.InstanceType))
				Expect(n.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeReserved))
				Expect(n.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, largeCapacityReservation.ID))

				NodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: xlargeCapacityReservation.ID}}
				Env.ExpectUpdated(NodeClass)
				Env.EventuallyExpectDrifted(nc)
			})
			It("should drift nodeclaim when the nodeclaim is demoted to on-demand", func() {
				capacityReservationID := Env.ExpectCapacityReservationCreated(1).ID
				DeferCleanup(func() {
					aws.ExpectCapacityReservationsCanceled(Env.Context, Env.EC2API, capacityReservationID)
				})

				NodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: capacityReservationID}}
				// Prevent drift from being executed by marking the pod as do-not-disrupt. Without this, the nodeclaim may be replaced
				// in-between polling intervals for the eventually block.
				pod := coretest.Pod(coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							karpv1.DoNotDisruptAnnotationKey: "true",
						},
					},
				})
				Env.ExpectCreated(NodePool, NodeClass, pod)

				nc := Env.EventuallyExpectLaunchedNodeClaimCount("==", 1)[0]
				req, ok := lo.Find(nc.Spec.Requirements, func(req karpv1.NodeSelectorRequirementWithMinValues) bool {
					return req.Key == v1.LabelCapacityReservationID
				})
				Expect(ok).To(BeTrue())
				Expect(req.Values).To(ConsistOf(capacityReservationID))
				n := Env.EventuallyExpectNodeCount("==", 1)[0]

				aws.ExpectCapacityReservationsCanceled(Env.Context, Env.EC2API, capacityReservationID)

				// The NodeClaim capacity reservation controller runs once every minute, we'll give a little extra time to avoid
				// a failure from a small delay, but the capacity type label should be updated and the reservation-id label should
				// be removed within a minute of the reservation being canceled.
				Eventually(func(g Gomega) {
					updatedNodeClaim := &karpv1.NodeClaim{}
					g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nc), updatedNodeClaim)).To(BeNil())
					g.Expect(updatedNodeClaim.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
					g.Expect(updatedNodeClaim.Labels).ToNot(HaveKey(v1.LabelCapacityReservationID))

					updatedNode := &corev1.Node{}
					g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(n), updatedNode)).To(BeNil())
					g.Expect(updatedNodeClaim.Labels).To(HaveKeyWithValue(karpv1.CapacityTypeLabelKey, karpv1.CapacityTypeOnDemand))
					g.Expect(updatedNodeClaim.Labels).ToNot(HaveKey(v1.LabelCapacityReservationID))
				}).WithTimeout(75 * time.Second).Should(Succeed())

				// Since the nodeclaim is only compatible with reserved instances, we should drift the node when it's demoted to on-demand
				Env.EventuallyExpectDrifted(nc)
			})
			It("should terminate the drifted nodeclaim before launching its replacement when the reservation is full", func() {
				capacityReservationID := Env.ExpectCapacityReservationCreated(1).ID
				DeferCleanup(func() {
					aws.ExpectCapacityReservationsCanceled(Env.Context, Env.EC2API, capacityReservationID)
				})

				NodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: capacityReservationID}}
				Env.ExpectCreated(dep, NodeClass, NodePool)
				pod := Env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
				nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
				node := Env.ExpectCreatedNodeCount("==", 1)[0]
				Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, capacityReservationID))

				NodeClass.Spec.Tags = lo.Assign(NodeClass.Spec.Tags, map[string]string{"test-key": "test-value"})
				Env.ExpectUpdated(NodeClass)
				Env.EventuallyExpectDrifted(nodeClaim)

				delete(pod.Annotations, karpv1.DoNotDisruptAnnotationKey)
				Env.ExpectUpdated(pod)
				Env.EventuallyExpectNotFound(pod, nodeClaim, node)
				Env.EventuallyExpectHealthyPodCount(selector, numPods)
			})
			// limits.nodes above replicas leaves headroom, but a reserved-only static pool can't launch a replacement while
			// its reservation is full, so static drift must terminate first rather than retry a replacement that never launches
			It("should terminate the drifted static nodeclaim before launching its replacement when the reservation is full", func() {
				capacityReservationID := Env.ExpectCapacityReservationCreated(1).ID
				DeferCleanup(func() {
					aws.ExpectCapacityReservationsCanceled(Env.Context, Env.EC2API, capacityReservationID)
				})

				NodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{ID: capacityReservationID}}
				NodePool.Spec.Replicas = lo.ToPtr(int64(1))
				NodePool.Spec.Limits = karpv1.Limits{resources.Node: resource.MustParse("2")}
				NodePool.Spec.Disruption.Budgets = []karpv1.Budget{{Nodes: "100%"}}
				Env.ExpectCreated(NodeClass, NodePool)
				nodeClaim := Env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
				node := Env.EventuallyExpectInitializedNodeCount("==", 1)[0]
				Expect(node.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, capacityReservationID))

				// Hold the node so the window between the drifted nodeclaim's deletion and its refill stays observable
				node.Finalizers = append(node.Finalizers, common.TestingFinalizer)
				Env.ExpectUpdated(node)
				NodeClass.Spec.Tags = lo.Assign(NodeClass.Spec.Tags, map[string]string{"test-key": "test-value"})
				Env.ExpectUpdated(NodeClass)

				listNodeClaimNames := func(g Gomega) []string {
					nodeClaims := &karpv1.NodeClaimList{}
					g.Expect(Env.Client.List(Env, nodeClaims, client.MatchingLabels{karpv1.NodePoolLabelKey: NodePool.Name})).To(Succeed())
					return lo.Map(nodeClaims.Items, func(nc karpv1.NodeClaim, _ int) string { return nc.Name })
				}
				// Replace-first would create a second nodeclaim that fails to launch into the full reservation
				Eventually(func(g Gomega) {
					g.Expect(Env.Client.Get(Env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
					g.Expect(nodeClaim.DeletionTimestamp.IsZero()).To(BeFalse())
					g.Expect(listNodeClaimNames(g)).To(ConsistOf(nodeClaim.Name))
				}).Should(Succeed())
				Consistently(func(g Gomega) {
					g.Expect(listNodeClaimNames(g)).To(ConsistOf(nodeClaim.Name))
				}, 30*time.Second, 5*time.Second).Should(Succeed())

				Expect(Env.ExpectTestingFinalizerRemoved(node)).To(Succeed())
				Env.EventuallyExpectNotFound(nodeClaim, node)
				replacement := Env.EventuallyExpectInitializedNodeCount("==", 1)[0]
				Expect(replacement.Labels).To(HaveKeyWithValue(v1.LabelCapacityReservationID, capacityReservationID))
			})
		})
	})
}
