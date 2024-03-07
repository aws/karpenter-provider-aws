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

package integration_test

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	awsenv "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AMI", func() {
	var customAMI string
	BeforeEach(func() {
		customAMI = env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
	})

	It("should use the AMI defined by the AMI Selector Terms", func() {
		pod := coretest.Pod()
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				ID: customAMI,
			},
		}
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should use the most recent AMI when discovering multiple", func() {
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: aws.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/amazon-eks-node-1.23-v20230322/image_id"),
		})
		Expect(err).To(BeNil())
		oldCustomAMI := *parameter.Parameter.Value
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				ID: customAMI,
			},
			{
				ID: oldCustomAMI,
			},
		}
		nodeClass.Spec.UserData = aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should support AMI Selector Terms for Name but fail with incorrect owners", func() {
		output, err := env.EC2API.DescribeImages(&ec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(customAMI)},
		})
		Expect(err).To(BeNil())
		Expect(output.Images).To(HaveLen(1))
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				Name:  *output.Images[0].Name,
				Owner: "fakeOwnerValue",
			},
		}
		nodeClass.Spec.UserData = aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.ExpectCreatedNodeCount("==", 0)
		Expect(pod.Spec.NodeName).To(Equal(""))
	})
	It("should support ami selector Name with default owners", func() {
		output, err := env.EC2API.DescribeImages(&ec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(customAMI)},
		})
		Expect(err).To(BeNil())
		Expect(output.Images).To(HaveLen(1))

		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				Name: *output.Images[0].Name,
			},
		}
		nodeClass.Spec.UserData = aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should support ami selector ids", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				ID: customAMI,
			},
		}
		nodeClass.Spec.UserData = aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})

	Context("AMIFamily", func() {
		It("should provision a node using the AL2 family", func() {
			pod := coretest.Pod()
			env.ExpectCreated(nodeClass, nodePool, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision a node using the AL2023 family", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2023
			pod := coretest.Pod()
			env.ExpectCreated(nodeClass, nodePool, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision a node using the Bottlerocket family", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			pod := coretest.Pod()
			env.ExpectCreated(nodeClass, nodePool, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision a node using the Ubuntu family", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyUbuntu
			// TODO (jmdeal@): remove once 22.04 AMIs are supported
			if env.GetK8sVersion(0) == "1.29" {
				nodeClass.Spec.AMISelectorTerms = lo.Map([]string{
					"/aws/service/canonical/ubuntu/eks/20.04/1.28/stable/current/amd64/hvm/ebs-gp2/ami-id",
					"/aws/service/canonical/ubuntu/eks/20.04/1.28/stable/current/arm64/hvm/ebs-gp2/ami-id",
				}, func(arg string, _ int) v1beta1.AMISelectorTerm {
					parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{Name: lo.ToPtr(arg)})
					Expect(err).To(BeNil())
					return v1beta1.AMISelectorTerm{ID: *parameter.Parameter.Value}
				})
			}
			// TODO: remove requirements after Ubuntu fixes bootstrap script issue w/
			// new instance types not included in the max-pods.txt file. (https://github.com/aws/karpenter-provider-aws/issues/4472)
			nodePool = coretest.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceFamily,
						Operator: v1.NodeSelectorOpNotIn,
						Values:   awsenv.ExcludedInstanceFamilies,
					},
				},
			)
			pod := coretest.Pod()
			env.ExpectCreated(nodeClass, nodePool, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support Custom AMIFamily with AMI Selectors", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: customAMI,
				},
			}
			nodeClass.Spec.UserData = aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", env.ClusterName))
			pod := coretest.Pod()

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)

			env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should have the EC2NodeClass status for AMIs using wildcard", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Name: "*",
				},
			}
			env.ExpectCreated(nodeClass)
			nc := EventuallyExpectAMIsToExist(nodeClass)
			Expect(len(nc.Status.AMIs)).To(BeNumerically("<", 10))
		})
		It("should have the EC2NodeClass status for AMIs using tags", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: customAMI,
				},
			}
			env.ExpectCreated(nodeClass)
			nc := EventuallyExpectAMIsToExist(nodeClass)
			Expect(len(nc.Status.AMIs)).To(BeNumerically("==", 1))
			Expect(nc.Status.AMIs[0].ID).To(Equal(customAMI))
		})
	})

	Context("UserData", func() {
		It("should merge UserData contents for AL2 AMIFamily", func() {
			content, err := os.ReadFile("testdata/al2_userdata_input.sh")
			Expect(err).ToNot(HaveOccurred())
			nodeClass.Spec.UserData = aws.String(string(content))
			nodePool.Spec.Template.Spec.Taints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
			nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}
			pod := coretest.Pod(coretest.PodOptions{Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}}})

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			Expect(env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
			))
			actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
			Expect(err).ToNot(HaveOccurred())
			// Since the node has joined the cluster, we know our bootstrapping was correct.
			// Just verify if the UserData contains our custom content too, rather than doing a byte-wise comparison.
			Expect(string(actualUserData)).To(ContainSubstring("Running custom user data script"))
		})
		It("should merge non-MIME UserData contents for AL2 AMIFamily", func() {
			content, err := os.ReadFile("testdata/al2_no_mime_userdata_input.sh")
			Expect(err).ToNot(HaveOccurred())
			nodeClass.Spec.UserData = aws.String(string(content))
			nodePool.Spec.Template.Spec.Taints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
			nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}
			pod := coretest.Pod(coretest.PodOptions{Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}}})

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			Expect(env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
			))
			actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
			Expect(err).ToNot(HaveOccurred())
			// Since the node has joined the cluster, we know our bootstrapping was correct.
			// Just verify if the UserData contains our custom content too, rather than doing a byte-wise comparison.
			Expect(string(actualUserData)).To(ContainSubstring("Running custom user data script"))
		})
		It("should merge UserData contents for Bottlerocket AMIFamily", func() {
			content, err := os.ReadFile("testdata/br_userdata_input.sh")
			Expect(err).ToNot(HaveOccurred())
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			nodeClass.Spec.UserData = aws.String(string(content))
			nodePool.Spec.Template.Spec.Taints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
			nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}
			pod := coretest.Pod(coretest.PodOptions{Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}}})

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			Expect(env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
			))
			actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(actualUserData)).To(ContainSubstring("kube-api-qps = 30"))
		})
		It("should merge UserData contents for Windows AMIFamily", func() {
			env.ExpectWindowsIPAMEnabled()
			DeferCleanup(func() {
				env.ExpectWindowsIPAMDisabled()
			})

			content, err := os.ReadFile("testdata/windows_userdata_input.ps1")
			Expect(err).ToNot(HaveOccurred())
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyWindows2022
			nodeClass.Spec.UserData = aws.String(string(content))
			nodePool.Spec.Template.Spec.Taints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
			nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}

			// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.* ENI data (https://github.com/aws/karpenter-provider-aws/issues/4472)
			nodePool = coretest.ReplaceRequirements(nodePool,
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceFamily,
						Operator: v1.NodeSelectorOpNotIn,
						Values:   awsenv.ExcludedInstanceFamilies,
					},
				},
				corev1beta1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: v1.NodeSelectorRequirement{
						Key:      v1.LabelOSStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{string(v1.Windows)},
					},
				},
			)
			pod := coretest.Pod(coretest.PodOptions{
				Image: awsenv.WindowsDefaultImage,
				NodeSelector: map[string]string{
					v1.LabelOSStable:     string(v1.Windows),
					v1.LabelWindowsBuild: "10.0.20348",
				},
				Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}},
			})

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthyWithTimeout(time.Minute*15, pod) // Wait 15 minutes because Windows nodes/containers take longer to spin up
			Expect(env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
				v1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
			))
			actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(actualUserData)).To(ContainSubstring("Write-Host \"Running custom user data script\""))
			Expect(string(actualUserData)).To(ContainSubstring("[string]$EKSBootstrapScriptFile = \"$env:ProgramFiles\\Amazon\\EKS\\Start-EKSBootstrap.ps1\""))
		})
	})
})

//nolint:unparam
func getInstanceAttribute(nodeName string, attribute string) *ec2.DescribeInstanceAttributeOutput {
	var node v1.Node
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: nodeName}, &node)).To(Succeed())
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instanceAttribute, err := env.EC2API.DescribeInstanceAttribute(&ec2.DescribeInstanceAttributeInput{
		InstanceId: aws.String(instanceID),
		Attribute:  aws.String(attribute),
	})
	Expect(err).ToNot(HaveOccurred())
	return instanceAttribute
}

func EventuallyExpectAMIsToExist(nodeClass *v1beta1.EC2NodeClass) *v1beta1.EC2NodeClass {
	nc := &v1beta1.EC2NodeClass{}
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClass), nc)).To(Succeed())
		g.Expect(nc.Status.AMIs).ToNot(BeNil())
	}).WithTimeout(30 * time.Second).Should(Succeed())
	return nc
}
