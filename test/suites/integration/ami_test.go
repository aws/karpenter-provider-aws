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
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var customAMI string

var _ = Describe("AMI", func() {
	BeforeEach(func() {
		customAMI = selectCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
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
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
	})
	It("should use the most recent AMI when discovering multiple", func() {
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: aws.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/amazon-eks-node-1.23-v20221101/image_id"),
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
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
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
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
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
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
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
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
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
			provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
			pod := test.Pod()

			env.ExpectCreated(pod, provider, provisioner)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)

			env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
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
				ProviderRef:   &v1alpha5.ProviderRef{Name: provider.Name},
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
				ProviderRef:   &v1alpha5.ProviderRef{Name: provider.Name},
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
	})
})

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

func selectCustomAMI(amiPath string, versionOffset int) string {
	serverVersion, err := env.KubeClient.Discovery().ServerVersion()
	Expect(err).To(BeNil())
	minorVersion, err := strconv.Atoi(strings.TrimSuffix(serverVersion.Minor, "+"))
	Expect(err).To(BeNil())
	// Choose a minor version one lesser than the server's minor version. This ensures that we choose an AMI for
	// this test that wouldn't be selected as Karpenter's SSM default (therefore avoiding false positives), and also
	// ensures that we aren't violating version skew.
	version := fmt.Sprintf("%s.%d", serverVersion.Major, minorVersion-versionOffset)
	parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf(amiPath, version)),
	})
	Expect(err).To(BeNil())
	return *parameter.Parameter.Value
}
