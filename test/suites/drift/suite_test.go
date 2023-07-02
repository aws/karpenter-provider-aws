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
	"strings"
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
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "true",
		})

	})
	It("should deprovision nodes that have drifted due to AMIs", func() {
		// choose an old static image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/recommended/image_id"),
		})
		Expect(err).To(BeNil())
		latestAMI := *parameter.Parameter.Value
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws-ids": latestAMI},
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
		// choose latest image
		parameter, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: awssdk.String("/aws/service/eks/optimized-ami/1.23/amazon-linux-2/recommended/image_id"),
		})
		Expect(err).To(BeNil())
		latestAMI := *parameter.Parameter.Value
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyCustom,
		},
			AMISelector: map[string]string{"aws-ids": latestAMI},
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
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"aws-ids": fmt.Sprintf("%s,%s", clusterSecurityGroupIDs, awssdk.StringValue(testSecurityGroup.GroupId))},
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
		provider.Spec.SecurityGroupSelector = map[string]string{"aws-ids": clusterSecurityGroupIDs}
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
	It("should deprovision nodes that have drifted due to subnets", func() {
		subnets := env.GetSubnetNameAndIds(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(subnets)).To(BeNumerically(">", 1))

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"aws-ids": subnets[0].ID},
		}})
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
		provider.Spec.SubnetSelector = map[string]string{"aws-ids": subnets[1].ID}
		env.ExpectCreatedOrUpdated(provider)

		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(node), node)).To(Succeed())
			g.Expect(node.Annotations).To(HaveKeyWithValue(v1alpha5.VoluntaryDisruptionAnnotationKey, v1alpha5.VoluntaryDisruptionDriftedAnnotationValue))
		}).Should(Succeed())

		delete(pod.Annotations, v1alpha5.DoNotEvictPodAnnotationKey)
		env.ExpectUpdated(pod)
		env.EventuallyExpectNotFound(pod, node)
	})
})
