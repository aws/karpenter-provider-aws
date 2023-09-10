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
	"strings"
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
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment
var customAMI string

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
})

var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Drift", Label("AWS"), func() {
	var pod *v1.Pod
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var provisioner *v1alpha5.Provisioner
	BeforeEach(func() {
		customAMI = env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
		nodeTemplate = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner = test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.CapacityTypeOnDemand}}},
			ProviderRef:  &v1alpha5.MachineTemplateRef{Name: nodeTemplate.Name},
		})
		// Add a do-not-evict pod so that we can check node metadata before we deprovision
		pod = test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha5.DoNotEvictPodAnnotationKey: "true",
				},
			},
		})
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "true",
		})
	})
	It("should deprovision nodes that have drifted due to AMIs", func() {
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/amazon-eks-node-1.23-v20230322/image_id"),
		})
		Expect(err).To(BeNil())
		oldCustomAMI := *parameter.Parameter.Value
		nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
		nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": oldCustomAMI}
		nodeTemplate.Spec.UserData = awssdk.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName))

		env.ExpectCreated(pod, nodeTemplate, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectNodeCount("==", 1)[0]
		nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": customAMI}
		env.ExpectCreatedOrUpdated(nodeTemplate)

		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, machine, node)
	})
	It("should not deprovision nodes that have drifted without the featureGate enabled", func() {
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "false",
		})
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/amazon-eks-node-1.23-v20230322/image_id"),
		})
		Expect(err).To(BeNil())
		oldCustomAMI := *parameter.Parameter.Value
		nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
		nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": oldCustomAMI}
		nodeTemplate.Spec.UserData = awssdk.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName))

		env.ExpectCreated(pod, nodeTemplate, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": customAMI}
		env.ExpectUpdated(nodeTemplate)

		// We should consistently get the same node existing for a minute
		Consistently(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), &v1.Node{})).To(Succeed())
		}).WithTimeout(time.Minute).Should(Succeed())
	})
	It("should deprovision nodes that have drifted due to securitygroup", func() {
		By("getting the cluster vpc id")
		output, err := env.EKSAPI.DescribeCluster(&eks.DescribeClusterInput{Name: awssdk.String(settings.FromContext(env.Context).ClusterName)})
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
							Value: awssdk.String(settings.FromContext(env.Context).ClusterName),
						},
						{
							Key:   awssdk.String(test.DiscoveryLabel),
							Value: awssdk.String(settings.FromContext(env.Context).ClusterName),
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
			securitygroups = env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
			testSecurityGroup, _ = lo.Find(securitygroups, func(sg aws.SecurityGroup) bool {
				return awssdk.StringValue(sg.GroupName) == "security-group-drift"
			})
			g.Expect(testSecurityGroup).ToNot(BeNil())
		}).Should(Succeed())

		By("creating a new provider with the new securitygroup")
		awsIDs := lo.Map(securitygroups, func(sg aws.SecurityGroup, _ int) string {
			if awssdk.StringValue(sg.GroupId) != awssdk.StringValue(testSecurityGroup.GroupId) {
				return awssdk.StringValue(sg.GroupId)
			}
			return ""
		})
		clusterSecurityGroupIDs := strings.Join(lo.WithoutEmpty(awsIDs), ",")
		nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": fmt.Sprintf("%s,%s", clusterSecurityGroupIDs, awssdk.StringValue(testSecurityGroup.GroupId))}

		env.ExpectCreated(pod, nodeTemplate, provisioner)
		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": clusterSecurityGroupIDs}
		env.ExpectCreatedOrUpdated(nodeTemplate)

		By("validating the drifted status condition has propagated")
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, machine, node)
	})
	It("should deprovision nodes that have drifted due to subnets", func() {
		subnets := env.GetSubnetNameAndIds(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(subnets)).To(BeNumerically(">", 1))

		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": subnets[0].ID}

		env.ExpectCreated(pod, nodeTemplate, provisioner)
		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": subnets[1].ID}
		env.ExpectCreatedOrUpdated(nodeTemplate)

		By("validating the drifted status condition has propagated")
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
	})
	DescribeTable("Provisioner Drift", func(fieldName string, provisionerOption test.ProvisionerOptions) {
		provisionerOption.ObjectMeta = provisioner.ObjectMeta
		updatedProvisioner := test.Provisioner(
			test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: nodeTemplate.Name}},
			provisionerOption,
		)

		env.ExpectCreated(pod, nodeTemplate, provisioner)
		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		env.ExpectCreatedOrUpdated(updatedProvisioner)

		By("validating the drifted status condition has propagated")
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
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
		Entry("Annotation Drift", "Annotation", test.ProvisionerOptions{Annotations: map[string]string{"keyAnnotationTest": "valueAnnotationTest"}}),
		Entry("Labels Drift", "Labels", test.ProvisionerOptions{Labels: map[string]string{"keyLabelTest": "valueLabelTest"}}),
		Entry("Taints Drift", "Taints", test.ProvisionerOptions{Taints: []v1.Taint{{Key: "example.com/another-taint-2", Effect: v1.TaintEffectPreferNoSchedule}}}),
		Entry("KubeletConfiguration Drift", "KubeletConfiguration", test.ProvisionerOptions{Kubelet: &v1alpha5.KubeletConfiguration{
			EvictionSoft:            map[string]string{"memory.available": "5%"},
			EvictionSoftGracePeriod: map[string]metav1.Duration{"memory.available": {Duration: time.Minute}},
		}}),
		Entry("Start-up Taints Drift", "Start-up Taint", test.ProvisionerOptions{StartupTaints: []v1.Taint{{Key: "example.com/another-taint-2", Effect: v1.TaintEffectPreferNoSchedule}}}),
		Entry("NodeRequirement Drift", "NodeRequirement", test.ProvisionerOptions{Requirements: []v1.NodeSelectorRequirement{{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.CapacityTypeSpot}}}}),
	)
	DescribeTable("AWSNodeTemplate Drift", func(fieldName string, nodeTemplateSpec v1alpha1.AWSNodeTemplateSpec) {
		if fieldName == "InstanceProfile" {
			nodeTemplateSpec.AWS.InstanceProfile = awssdk.String(fmt.Sprintf("KarpenterNodeInstanceProfile-Drift-%s", settings.FromContext(env.Context).ClusterName))
			ExpectInstanceProfileCreated(nodeTemplateSpec.AWS.InstanceProfile)
		}

		updatedAWSNodeTemplate := awstest.AWSNodeTemplate(*nodeTemplate.Spec.DeepCopy(), nodeTemplateSpec)
		updatedAWSNodeTemplate.ObjectMeta = nodeTemplate.ObjectMeta
		updatedAWSNodeTemplate.Annotations = map[string]string{v1alpha1.AnnotationNodeTemplateHash: updatedAWSNodeTemplate.Hash()}

		env.ExpectCreated(pod, nodeTemplate, provisioner)
		machine := env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthy(pod)

		env.ExpectCreatedOrUpdated(updatedAWSNodeTemplate)

		By("validating the drifted status condition has propagated")
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted)).ToNot(BeNil())
			g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
	},
		Entry("InstanceProfile Drift", "InstanceProfile", v1alpha1.AWSNodeTemplateSpec{}),
		Entry("UserData Drift", "UserData", v1alpha1.AWSNodeTemplateSpec{UserData: awssdk.String("#!/bin/bash\n/etc/eks/bootstrap.sh")}),
		Entry("Tags Drift", "Tags", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("MetadataOptions Drift", "MetadataOptions", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{LaunchTemplate: v1alpha1.LaunchTemplate{MetadataOptions: &v1alpha1.MetadataOptions{HTTPTokens: awssdk.String("required"), HTTPPutResponseHopLimit: awssdk.Int64(10)}}}}),
		Entry("BlockDeviceMappings Drift", "BlockDeviceMappings", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{LaunchTemplate: v1alpha1.LaunchTemplate{BlockDeviceMappings: []*v1alpha1.BlockDeviceMapping{
			{
				DeviceName: awssdk.String("/dev/xvda"),
				EBS: &v1alpha1.BlockDevice{
					VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					VolumeType: awssdk.String("gp3"),
					Encrypted:  awssdk.Bool(true),
				},
			}}}}}),
		Entry("DetailedMonitoring Drift", "DetailedMonitoring", v1alpha1.AWSNodeTemplateSpec{DetailedMonitoring: awssdk.Bool(true)}),
		Entry("AMIFamily Drift", "AMIFamily", v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{AMIFamily: awssdk.String(v1alpha1.AMIFamilyBottlerocket)}}),
	)
	Context("Drift Failure", func() {
		It("should not continue to drift if a node never registers", func() {
			// launch a new machine
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
			env.ExpectCreated(dep, nodeTemplate, provisioner)

			startingMachineState := env.EventuallyExpectCreatedMachineCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Drift the machine with bad configuration
			parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
				Name: awssdk.String("/aws/service/ami-amazon-linux-latest/amzn-ami-hvm-x86_64-ebs"),
			})
			Expect(err).ToNot(HaveOccurred())
			nodeTemplate.Spec.AMISelector = map[string]string{"aws::ids": *parameter.Parameter.Value}
			env.ExpectCreatedOrUpdated(nodeTemplate)

			// Should see the machine has drifted
			Eventually(func(g Gomega) {
				for _, machine := range startingMachineState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
					g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To get cordoned
			cordonedNodes := env.EventuallyExpectCordonedNodeCount("==", 1)

			// Drift should fail and the original node should be uncordoned
			// TODO: reduce timeouts when deprovisioning waits are factored out
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(cordonedNodes[0]), cordonedNodes[0]))
				g.Expect(cordonedNodes[0].Spec.Unschedulable).To(BeFalse())
			}).WithTimeout(11 * time.Minute).Should(Succeed())

			Eventually(func(g Gomega) {
				machines := &v1alpha5.MachineList{}
				g.Expect(env.Client.List(env, machines, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
				g.Expect(machines.Items).To(HaveLen(int(numPods)))
			}).WithTimeout(6 * time.Minute).Should(Succeed())

			// Expect all the Machines that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				machines := &v1alpha5.MachineList{}
				g.Expect(env.Client.List(env, machines, client.HasLabels{test.DiscoveryLabel})).To(Succeed())

				startingMachineUIDs := lo.Map(startingMachineState, func(m *v1alpha5.Machine, _ int) types.UID { return m.UID })
				machineUIDs := lo.Map(machines.Items, func(m v1alpha5.Machine, _ int) types.UID { return m.UID })
				g.Expect(sets.New(machineUIDs...).IsSuperset(sets.New(startingMachineUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
		It("should not continue to drift if a node registers but never becomes initialized", func() {
			// launch a new machine
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
			env.ExpectCreated(dep, nodeTemplate, provisioner)

			startingMachineState := env.EventuallyExpectCreatedMachineCount("==", int(numPods))
			env.EventuallyExpectCreatedNodeCount("==", int(numPods))

			// Drift the machine with bad configuration that never initializes
			provisioner.Spec.StartupTaints = []v1.Taint{{Key: "example.com/taint", Effect: v1.TaintEffectPreferNoSchedule}}
			env.ExpectCreatedOrUpdated(provisioner)

			// Should see the machine has drifted
			Eventually(func(g Gomega) {
				for _, machine := range startingMachineState {
					g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(machine), machine)).To(Succeed())
					g.Expect(machine.StatusConditions().GetCondition(v1alpha5.MachineDrifted).IsTrue()).To(BeTrue())
				}
			}).Should(Succeed())

			// Expect nodes To be cordoned
			cordonedNodes := env.EventuallyExpectCordonedNodeCount("==", 1)

			// Drift should fail and original node should be uncordoned
			// TODO: reduce timeouts when deprovisioning waits are factored outr
			Eventually(func(g Gomega) {
				g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(cordonedNodes[0]), cordonedNodes[0]))
				g.Expect(cordonedNodes[0].Spec.Unschedulable).To(BeFalse())
			}).WithTimeout(12 * time.Minute).Should(Succeed())

			// Expect that the new machine/node is kept around after the un-cordon
			nodeList := &v1.NodeList{}
			Expect(env.Client.List(env, nodeList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
			Expect(nodeList.Items).To(HaveLen(int(numPods) + 1))

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(env, machineList, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
			Expect(machineList.Items).To(HaveLen(int(numPods) + 1))

			// Expect all the Machines that existed on the initial provisioning loop are not removed
			Consistently(func(g Gomega) {
				machines := &v1alpha5.MachineList{}
				g.Expect(env.Client.List(env, machines, client.HasLabels{test.DiscoveryLabel})).To(Succeed())

				startingMachineUIDs := lo.Map(startingMachineState, func(m *v1alpha5.Machine, _ int) types.UID { return m.UID })
				machineUIDs := lo.Map(machines.Items, func(m v1alpha5.Machine, _ int) types.UID { return m.UID })
				g.Expect(sets.New(machineUIDs...).IsSuperset(sets.New(startingMachineUIDs...))).To(BeTrue())
			}, "2m").Should(Succeed())
		})
	})
})

func ExpectInstanceProfileCreated(instanceProfileName *string) {
	By("creating an instance profile")
	createInstanceProfile := &iam.CreateInstanceProfileInput{
		InstanceProfileName: instanceProfileName,
		Tags: []*iam.Tag{
			{
				Key:   awssdk.String(test.DiscoveryLabel),
				Value: awssdk.String(settings.FromContext(env.Context).ClusterName),
			},
		},
	}
	By("adding the karpenter role to new instance profile")
	_, err := env.IAMAPI.CreateInstanceProfile(createInstanceProfile)
	Expect(ignoreAlreadyExists(err)).ToNot(HaveOccurred())
	addInstanceProfile := &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: instanceProfileName,
		RoleName:            awssdk.String(fmt.Sprintf("KarpenterNodeRole-%s", settings.FromContext(env.Context).ClusterName)),
	}
	_, err = env.IAMAPI.AddRoleToInstanceProfile(addInstanceProfile)
	Expect(ignoreAlreadyContainsRole(err)).ToNot(HaveOccurred())
}

func ignoreAlreadyExists(err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "EntityAlreadyExists") {
			return nil
		}
	}
	return err
}

func ignoreAlreadyContainsRole(err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "Cannot exceed quota for InstanceSessionsPerInstanceProfile") {
			return nil
		}
	}

	return err
}
