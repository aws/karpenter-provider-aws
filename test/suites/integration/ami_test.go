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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
	awsenv "github.com/aws/karpenter/test/pkg/environment/aws"
)

var _ = Describe("AMI", func() {
	var customAMI string
	BeforeEach(func() {
		customAMI = env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
	})

	It("should use the AMI defined by the AMI Selector", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyAL2,
			},
			AMISelector: map[string]string{"aws-ids": customAMI},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
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
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws-ids": fmt.Sprintf("%s,%s", customAMI, oldCustomAMI)},
			UserData:    aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should support ami selector aws::name but fail with incorrect owners", func() {
		output, err := env.EC2API.DescribeImages(&ec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(customAMI)},
		})
		Expect(err).To(BeNil())
		Expect(output.Images).To(HaveLen(1))

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws::name": *output.Images[0].Name, "aws::owners": "fakeOwnerValue"},
			UserData:    aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
		})

		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.ExpectCreatedNodeCount("==", 0)
		Expect(pod.Spec.NodeName).To(Equal(""))
	})
	It("should support ami selector aws::name with default owners", func() {
		output, err := env.EC2API.DescribeImages(&ec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(customAMI)},
		})
		Expect(err).To(BeNil())
		Expect(output.Images).To(HaveLen(1))

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws::name": *output.Images[0].Name},
			UserData:    aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
		})

		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should support ami selector aws::ids", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws::ids": customAMI},
			UserData:    aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})

	Context("AMIFamily", func() {
		It("should provision a node using the AL2 family", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			})
			pod := test.Pod()
			env.ExpectCreated(provider, provisioner, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision a node using the Bottlerocket family", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyBottlerocket,
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			})
			pod := test.Pod()
			env.ExpectCreated(provider, provisioner, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should provision a node using the Ubuntu family", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyUbuntu,
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
				// TODO: remove requirements after Ubuntu fixes bootstrap script issue w/
				// new instance types not included in the max-pods.txt file. (https://github.com/aws/karpenter/issues/4472)
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceGeneration,
						Operator: v1.NodeSelectorOpLt,
						Values:   []string{"7"},
					},
					{
						Key:      v1.LabelOSStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{string(v1.Linux)},
					},
				},
			})
			pod := test.Pod()
			env.ExpectCreated(provider, provisioner, pod)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
		})
		It("should support Custom AMIFamily with AMI Selectors", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyCustom,
			},
				AMISelector: map[string]string{"aws-ids": customAMI},
				UserData:    aws.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
			})
			provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
			pod := test.Pod()

			env.ExpectCreated(pod, provider, provisioner)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)

			env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should have the AWSNodeTemplateStatus for AMIs using wildcard", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				},
				AMISelector: map[string]string{"aws::name": "*"},
			})

			env.ExpectCreated(provider)
			ant := EventuallyExpectAMIsToExist(provider)
			Expect(len(ant.Status.AMIs)).To(BeNumerically("<", 10))
		})
		It("should have the AWSNodeTemplateStatus for AMIs using tags", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				},
				AMISelector: map[string]string{"aws-ids": customAMI},
			})

			env.ExpectCreated(provider)
			ant := EventuallyExpectAMIsToExist(provider)

			Expect(len(ant.Status.AMIs)).To(BeNumerically("==", 1))
			Expect(ant.Status.AMIs[0].ID).To(Equal(customAMI))
		})
	})

	Context("UserData", func() {
		It("should merge UserData contents for AL2 AMIFamily", func() {
			content, err := os.ReadFile("testdata/al2_userdata_input.sh")
			Expect(err).ToNot(HaveOccurred())
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyAL2,
			},
				UserData: aws.String(string(content)),
			})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef:   &v1alpha5.MachineTemplateRef{Name: provider.Name},
				Taints:        []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}},
				StartupTaints: []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}},
			})
			pod := test.Pod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}}})

			env.ExpectCreated(pod, provider, provisioner)
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
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyAL2,
			},
				UserData: aws.String(string(content)),
			})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef:   &v1alpha5.MachineTemplateRef{Name: provider.Name},
				Taints:        []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}},
				StartupTaints: []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}},
			})
			pod := test.Pod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}}})

			env.ExpectCreated(pod, provider, provisioner)
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
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyBottlerocket,
			},
				UserData: aws.String(string(content)),
			})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef:   &v1alpha5.MachineTemplateRef{Name: provider.Name},
				Taints:        []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}},
				StartupTaints: []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}},
			})
			pod := test.Pod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}}})

			env.ExpectCreated(pod, provider, provisioner)
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
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyWindows2022,
			},
				UserData: aws.String(string(content)),
			})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef:   &v1alpha5.MachineTemplateRef{Name: provider.Name},
				Taints:        []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}},
				StartupTaints: []v1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelOSStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{string(v1.Windows)},
					},
					// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.* ENI data (https://github.com/aws/karpenter/issues/4472)
					{
						Key:      v1alpha1.LabelInstanceFamily,
						Operator: v1.NodeSelectorOpNotIn,
						Values:   []string{"m7a", "r7a", "c7a"},
					},
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"c", "m", "r"},
					},
					{
						Key:      v1alpha1.LabelInstanceGeneration,
						Operator: v1.NodeSelectorOpGt,
						Values:   []string{"2"},
					},
				},
			})
			pod := test.Pod(test.PodOptions{
				Image: awsenv.WindowsDefaultImage,
				NodeSelector: map[string]string{
					v1.LabelOSStable:     string(v1.Windows),
					v1.LabelWindowsBuild: "10.0.20348",
				},
				Tolerations: []v1.Toleration{{Key: "example.com", Operator: v1.TolerationOpExists}},
			})

			env.ExpectCreated(pod, provider, provisioner)
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

func EventuallyExpectAMIsToExist(provider *v1alpha1.AWSNodeTemplate) v1alpha1.AWSNodeTemplate {
	var ant v1alpha1.AWSNodeTemplate
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(provider), &ant)).To(Succeed())
		g.Expect(ant.Status.AMIs).ToNot(BeNil())
	}).WithTimeout(30 * time.Second).Should(Succeed())

	return ant
}
