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
	"sort"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/ssm"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment
var amdAMI string
var nodeClass *v1beta1.EC2NodeClass
var nodePool *corev1beta1.NodePool

func TestDrift(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Beta/Drift")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})

var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Beta/Drift", Label("AWS"), func() {
	var pod *v1.Pod
	BeforeEach(func() {
		amdAMI = env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
		// Add a do-not-disrupt pod so that we can check node metadata before we disrupt
		pod = test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					corev1beta1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		env.ExpectSettingsOverridden(v1.EnvVar{Name: "FEATURE_GATES", Value: "Drift=true"})
	})
	It("should disrupt nodes that have drifted due to AMIs", func() {
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/amazon-eks-node-1.23-v20230322/image_id"),
		})
		Expect(err).To(BeNil())
		oldCustomAMI := *parameter.Parameter.Value
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: oldCustomAMI}}
		nodeClass.Spec.UserData = awssdk.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectNodeCount("==", 1)[0]
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: amdAMI}}
		env.ExpectCreatedOrUpdated(nodeClass)

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
	})
	It("should return drifted if the AMI no longer matches the existing NodeClaims instance type", func() {
		version := env.GetK8sVersion(1)
		armParameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version)),
		})
		Expect(err).To(BeNil())
		armAMI := *armParameter.Parameter.Value
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: armAMI}}

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectNodeCount("==", 1)[0]
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: amdAMI}}
		env.ExpectCreatedOrUpdated(nodeClass)

		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted)).ToNot(BeNil())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
	})
	It("should not disrupt nodes that have drifted without the featureGate enabled", func() {
		version := env.GetK8sVersion(1)
		env.ExpectSettingsOverridden(v1.EnvVar{Name: "FEATURE_GATES", Value: "Drift=false"})
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/recommended/image_id", version)),
		})
		Expect(err).To(BeNil())
		oldCustomAMI := *parameter.Parameter.Value
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: oldCustomAMI}}
		nodeClass.Spec.UserData = awssdk.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: amdAMI}}
		env.ExpectUpdated(nodeClass)

		// We should consistently get the same node existing for a minute
		Consistently(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), &v1.Node{})).To(Succeed())
		}).WithTimeout(time.Minute).Should(Succeed())
	})
	It("should disrupt nodes that have drifted due to securitygroup", func() {
		By("getting the cluster vpc id")
		output, err := env.EKSAPI.DescribeCluster(&eks.DescribeClusterInput{Name: awssdk.String(env.ClusterName)})
		Expect(err).To(BeNil())

		By("creating new security group")
		createSecurityGroup := &ec2.CreateSecurityGroupInput{
			GroupName:   awssdk.String("security-group-drift"),
			Description: awssdk.String("End-to-end Drift Test, should delete after drift test is completed"),
			VpcId:       output.Cluster.ResourcesVpcConfig.VpcId,
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: awssdk.String("security-group"),
					Tags: []*ec2.Tag{
						{
							Key:   awssdk.String("karpenter.sh/discovery"),
							Value: awssdk.String(env.ClusterName),
						},
						{
							Key:   awssdk.String(test.DiscoveryLabel),
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
		_, _ = env.EC2API.CreateSecurityGroup(createSecurityGroup)

		By("looking for security groups")
		var securitygroups []aws.SecurityGroup
		var testSecurityGroup aws.SecurityGroup
		Eventually(func(g Gomega) {
			securitygroups = env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
			testSecurityGroup, _ = lo.Find(securitygroups, func(sg aws.SecurityGroup) bool {
				return awssdk.StringValue(sg.GroupName) == "security-group-drift"
			})
			g.Expect(testSecurityGroup).ToNot(BeNil())
		}).Should(Succeed())

		By("creating a new provider with the new securitygroup")
		awsIDs := lo.FilterMap(securitygroups, func(sg aws.SecurityGroup, _ int) (string, bool) {
			if awssdk.StringValue(sg.GroupId) != awssdk.StringValue(testSecurityGroup.GroupId) {
				return awssdk.StringValue(sg.GroupId), true
			}
			return "", false
		})
		sgTerms := []v1beta1.SecurityGroupSelectorTerm{{ID: awssdk.StringValue(testSecurityGroup.GroupId)}}
		for _, id := range awsIDs {
			sgTerms = append(sgTerms, v1beta1.SecurityGroupSelectorTerm{ID: id})
		}
		nodeClass.Spec.SecurityGroupSelectorTerms = sgTerms

		env.ExpectCreated(pod, nodeClass, nodePool)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		sgTerms = lo.Reject(sgTerms, func(t v1beta1.SecurityGroupSelectorTerm, _ int) bool {
			return t.ID == awssdk.StringValue(testSecurityGroup.GroupId)
		})
		nodeClass.Spec.SecurityGroupSelectorTerms = sgTerms
		env.ExpectCreatedOrUpdated(nodeClass)

		By("validating the drifted status condition has propagated")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted)).ToNot(BeNil())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, nodeClaim, node)
	})
	It("should disrupt nodes that have drifted due to subnets", func() {
		subnets := env.GetSubnetNameAndIds(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(subnets)).To(BeNumerically(">", 1))

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{{ID: subnets[0].ID}}

		env.ExpectCreated(pod, nodeClass, nodePool)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{{ID: subnets[1].ID}}
		env.ExpectCreatedOrUpdated(nodeClass)

		By("validating the drifted status condition has propagated")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted)).ToNot(BeNil())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
	})
	DescribeTable("NodePool Drift", func(fieldName string, nodePoolOption corev1beta1.NodeClaimTemplate) {
		updatedNodePool := test.NodePool(
			corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
						},
					},
				},
			},
			corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: nodePoolOption,
				},
			},
		)
		updatedNodePool.ObjectMeta = nodePool.ObjectMeta

		env.ExpectCreated(pod, nodeClass, nodePool)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		env.ExpectCreatedOrUpdated(updatedNodePool)

		By("validating the drifted status condition has propagated")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted)).ToNot(BeNil())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)

		// Nodes will need to have the start-up taint removed before the node can be considered as initialized
		if fieldName == "Start-up Taint" {
			nodes := env.EventuallyExpectCreatedNodeCount("==", 2)
			sort.Slice(nodes, func(i int, j int) bool {
				return nodes[i].CreationTimestamp.Before(&nodes[j].CreationTimestamp)
			})
			nodeTwo := nodes[1]
			// Remove the startup taints from the new nodes to initialize them
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeTwo), nodeTwo)).To(Succeed())
				stored := nodeTwo.DeepCopy()
				nodeTwo.Spec.Taints = lo.Reject(nodeTwo.Spec.Taints, func(t v1.Taint, _ int) bool { return t.Key == "example.com/another-taint-2" })
				g.Expect(env.Client.Patch(env.Context, nodeTwo, client.MergeFrom(stored))).To(Succeed())
			}).Should(Succeed())
		}

		env.EventuallyExpectNotFound(pod, node)
	},
		Entry("Annotation Drift", "Annotation", corev1beta1.NodeClaimTemplate{
			ObjectMeta: corev1beta1.ObjectMeta{
				Annotations: map[string]string{"keyAnnotationTest": "valueAnnotationTest"},
			},
		}),
		Entry("Labels Drift", "Labels", corev1beta1.NodeClaimTemplate{
			ObjectMeta: corev1beta1.ObjectMeta{
				Labels: map[string]string{"keyLabelTest": "valueLabelTest"},
			},
		}),
		Entry("Taints Drift", "Taints", corev1beta1.NodeClaimTemplate{
			Spec: corev1beta1.NodeClaimSpec{
				Taints: []v1.Taint{{Key: "example.com/another-taint-2", Effect: v1.TaintEffectPreferNoSchedule}},
			},
		}),
		Entry("KubeletConfiguration Drift", "KubeletConfiguration", corev1beta1.NodeClaimTemplate{
			Spec: corev1beta1.NodeClaimSpec{
				Kubelet: &corev1beta1.KubeletConfiguration{
					EvictionSoft:            map[string]string{"memory.available": "5%"},
					EvictionSoftGracePeriod: map[string]metav1.Duration{"memory.available": {Duration: time.Minute}},
				},
			},
		}),
		Entry("Start-up Taints Drift", "Start-up Taint", corev1beta1.NodeClaimTemplate{
			Spec: corev1beta1.NodeClaimSpec{
				StartupTaints: []v1.Taint{{Key: "example.com/another-taint-2", Effect: v1.TaintEffectPreferNoSchedule}},
			},
		}),
		Entry("NodeRequirement Drift", "NodeRequirement", corev1beta1.NodeClaimTemplate{
			Spec: corev1beta1.NodeClaimSpec{
				Requirements: []v1.NodeSelectorRequirement{{Key: corev1beta1.CapacityTypeLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{corev1beta1.CapacityTypeSpot}}},
			},
		}),
	)
	DescribeTable("EC2NodeClass Drift", func(fieldName string, nodeClassSpec v1beta1.EC2NodeClassSpec) {
		updatedNodeClass := awstest.EC2NodeClass(v1beta1.EC2NodeClass{Spec: *nodeClass.Spec.DeepCopy()}, v1beta1.EC2NodeClass{Spec: nodeClassSpec})
		updatedNodeClass.ObjectMeta = nodeClass.ObjectMeta
		updatedNodeClass.Annotations = map[string]string{v1beta1.AnnotationNodeClassHash: updatedNodeClass.Hash()}

		env.ExpectCreated(pod, nodeClass, nodePool)
		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		env.ExpectCreatedOrUpdated(updatedNodeClass)

		By("validating the drifted status condition has propagated")
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted)).ToNot(BeNil())
			g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, corev1beta1.DoNotDisruptAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
	},
		Entry("UserData Drift", "UserData", v1beta1.EC2NodeClassSpec{UserData: awssdk.String("#!/bin/bash\n/etc/eks/bootstrap.sh")}),
		Entry("Tags Drift", "Tags", v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}),
		Entry("MetadataOptions Drift", "MetadataOptions", v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPTokens: awssdk.String("required"), HTTPPutResponseHopLimit: awssdk.Int64(10)}}),
		Entry("BlockDeviceMappings Drift", "BlockDeviceMappings", v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{
			{
				DeviceName: awssdk.String("/dev/xvda"),
				EBS: &v1beta1.BlockDevice{
					VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					VolumeType: awssdk.String("gp3"),
					Encrypted:  awssdk.Bool(true),
				},
			}}}),
		Entry("DetailedMonitoring Drift", "DetailedMonitoring", v1beta1.EC2NodeClassSpec{DetailedMonitoring: awssdk.Bool(true)}),
		Entry("AMIFamily Drift", "AMIFamily", v1beta1.EC2NodeClassSpec{AMIFamily: awssdk.String(v1beta1.AMIFamilyBottlerocket)}),
	)
	Context("Drift Failure", func() {
		It("should not continue to drift if a node never registers", func() {
			// launch a new nodeClaim
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			env.ExpectCreated(dep, nodeClass, nodePool)

			startingNodeClaimState := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Drift the nodeClaim with bad configuration
			parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
				Name: awssdk.String("/aws/service/ami-amazon-linux-latest/amzn-ami-hvm-x86_64-ebs"),
			})
			Expect(err).ToNot(HaveOccurred())
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: *parameter.Parameter.Value}}
			env.ExpectCreatedOrUpdated(nodeClass)

			// Should see the nodeClaim has drifted
			Eventually(func(g Gomega) {
				for _, nodeClaim := range startingNodeClaimState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
					g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To get cordoned
			cordonedNodes := env.EventuallyExpectCordonedNodeCount("==", 1)

			// Drift should fail and the original node should be uncordoned
			// TODO: reduce timeouts when disruption waits are factored out
			env.EventuallyExpectNodesUncordonedWithTimeout(11*time.Minute, cordonedNodes...)

			// We give another 6 minutes here to handle the deletion at the 15m registration timeout
			Eventually(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
				g.Expect(nodeClaims.Items).To(HaveLen(int(numPods)))
			}).WithTimeout(6 * time.Minute).Should(Succeed())

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{test.DiscoveryLabel})).To(Succeed())

				startingNodeClaimUIDs := lo.Map(startingNodeClaimState, func(nc *corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				nodeClaimUIDs := lo.Map(nodeClaims.Items, func(nc corev1beta1.NodeClaim, _ int) types.UID { return nc.UID })
				g.Expect(sets.New(nodeClaimUIDs...).IsSuperset(sets.New(startingNodeClaimUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not continue to drift if a node registers but never becomes initialized", func() {
			// launch a new nodeClaim
			var numPods int32 = 2
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: 2,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inflate"}},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						TopologyKey: v1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "inflate"},
						}},
					},
				},
			})
			env.ExpectCreated(dep, nodeClass, nodePool)

			startingNodeClaimState := env.EventuallyExpectCreatedNodeClaimCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Drift the nodeClaim with bad configuration that never initializes
			nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "example.com/taint", Effect: v1.TaintEffectPreferNoSchedule}}
			env.ExpectCreatedOrUpdated(nodePool)

			// Should see the nodeClaim has drifted
			Eventually(func(g Gomega) {
				for _, nodeClaim := range startingNodeClaimState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
					g.Expect(nodeClaim.StatusConditions().GetCondition(corev1beta1.Drifted).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To be cordoned
			cordonedNodes := env.EventuallyExpectCordonedNodeCount("==", 1)

			// Drift should fail and original node should be uncordoned
			// TODO: reduce timeouts when disruption waits are factored out
			env.EventuallyExpectNodesUncordonedWithTimeout(11*time.Minute, cordonedNodes...)

			// Expect that the new nodeClaim/node is kept around after the un-cordon
			nodeList := &v1.NodeList{}
			Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
			Expect(nodeList.Items).To(HaveLen(int(numPods) + 1))

			nodeClaimList := &corev1beta1.NodeClaimList{}
			Expect(env.Client.List(env, nodeClaimList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
			Expect(nodeClaimList.Items).To(HaveLen(int(numPods) + 1))

			// Expect all the NodeClaims that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				nodeClaims := &corev1beta1.NodeClaimList{}
				g.Expect(env.Client.List(env, nodeClaims, client.HasLabels{test.DiscoveryLabel})).To(Succeed())

				startingNodeClaimUIDs := lo.Map(startingNodeClaimState, func(m *corev1beta1.NodeClaim, _ int) types.UID { return m.UID })
				nodeClaimUIDs := lo.Map(nodeClaims.Items, func(m corev1beta1.NodeClaim, _ int) types.UID { return m.UID })
				g.Expect(sets.New(nodeClaimUIDs...).IsSuperset(sets.New(startingNodeClaimUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
	})
})
