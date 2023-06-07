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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
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
	RunSpecs(t, "Drift")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
})

var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Drift", Label("AWS"), func() {
	BeforeEach(func() {
		customAMI = env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
	})
	It("should deprovision nodes that have drifted due to AMIs", func() {
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "true",
		})
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/amazon-eks-node-1.23-v20230322/image_id"),
		})
		Expect(err).To(BeNil())
		oldCustomAMI := *parameter.Parameter.Value
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws-ids": oldCustomAMI},
			UserData:    awssdk.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})

		// Add a do-not-evict pod so that we can check node metadata before we deprovision
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha5.DoNotEvictPodAnnotationKey: "true",
				},
			},
		})

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		provider.Spec.AMISelector = map[string]string{"aws-ids": customAMI}
		env.ExpectCreatedOrUpdated(provider)

		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(node.Annotations).To(HaveKeyWithValue(v1alpha5.VoluntaryDisruptionAnnotationKey, v1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
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
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws-ids": oldCustomAMI},
			UserData:    awssdk.String(fmt.Sprintf("#!/bin/bash\n/etc/eks/bootstrap.sh '%s'", settings.FromContext(env.Context).ClusterName)),
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})

		// Add a do-not-evict pod so that we can check node metadata before we deprovision
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha5.DoNotEvictPodAnnotationKey: "true",
				},
			},
		})

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.CreatedNodes()[0]
		provider.Spec.AMISelector = map[string]string{"aws-ids": customAMI}
		env.ExpectUpdated(provider)

		// We should consistently get the same node existing for a minute
		Consistently(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(node), &v1.Node{})).To(Succeed())
		}).WithTimeout(time.Minute).Should(Succeed())
	})
	It("should deprovision nodes that have drifted due to securitygroup", func() {
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "true",
		})
		By("Getting the Cluster VPCID")
		output, err := env.EKSAPI.DescribeCluster(&eks.DescribeClusterInput{Name: awssdk.String(settings.FromContext(env.Context).ClusterName)})
		Expect(err).To(BeNil())

		By("creating a new securitygroup")
		securitygroup := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(securitygroup)).To(BeNumerically("==", 2))

		createSecurityGroup := &ec2.CreateSecurityGroupInput{
			GroupName:   awssdk.String("security-group-drift"),
			Description: awssdk.String("End-to-end Drift Test, should delete after drift test is completed"),
			VpcId:       output.Cluster.ResourcesVpcConfig.VpcId,
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: awssdk.String("security-group"),
					Tags: []*ec2.Tag{
						{
							Key:   awssdk.String("security-group-drift"),
							Value: awssdk.String(settings.FromContext(env.Context).ClusterName),
						},
					},
				},
			},
		}
		newSecurityGroup, err := env.EC2API.CreateSecurityGroup(createSecurityGroup)
		Expect(err).To(BeNil())
		DeferCleanup(func() {
			By("deleting the new securitygroup")
			// // Need to make sure that all instance launched with the security group are terminated
			EventuallyWithOffset(1, func(g Gomega) {
				output, err := env.EC2API.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
					Filters: []*ec2.Filter{{Name: awssdk.String("group-id"), Values: []*string{newSecurityGroup.GroupId}}},
				})
				g.Expect(err).To(BeNil())
				_, err = env.EC2API.ModifyNetworkInterfaceAttribute(&ec2.ModifyNetworkInterfaceAttributeInput{
					NetworkInterfaceId: output.NetworkInterfaces[0].NetworkInterfaceId,
					Groups:             lo.Map(securitygroup, func(sg aws.SecurityGroup, _ int) *string { return sg.GroupId }),
				})
				g.Expect(err).To(BeNil())
				_, err = env.EC2API.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
					GroupId: newSecurityGroup.GroupId,
				})
				g.Expect(err).To(BeNil())
			}).Should(Succeed())
		})
		By("creating a new provider with the new securitygroup")
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"aws-ids": fmt.Sprintf("%s,%s,%s", *securitygroup[0].GroupId, *securitygroup[1].GroupId, awssdk.StringValue(newSecurityGroup.GroupId))},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})

		// Add a do-not-evict pod so that we can check node metadata before we deprovision
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha5.DoNotEvictPodAnnotationKey: "true",
				},
			},
		})

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		By("updating the provider securitygroup")
		node := env.Monitor.CreatedNodes()[0]
		provider.Spec.SecurityGroupSelector = map[string]string{"aws-ids": fmt.Sprintf("%s,%s", *securitygroup[0].GroupId, *securitygroup[1].GroupId)}
		env.ExpectCreatedOrUpdated(provider)

		By("checking the node metadata")
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(node.Annotations).To(HaveKeyWithValue(v1alpha5.VoluntaryDisruptionAnnotationKey, v1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
	})
})
