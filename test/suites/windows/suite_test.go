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

package windows_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var vpcCNIConfigMap = &v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "amazon-vpc-cni",
		Namespace: "kube-system",
	},
	Data: map[string]string{
		"enable-windows-ipam": "true",
	},
}

var env *aws.Environment

func TestWindows(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Integration")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.ForceCleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Windows", func() {
	BeforeEach(func() {
		env.ExpectCreatedOrUpdated(vpcCNIConfigMap)
	})
	AfterEach(func() {
		env.ExpectDeleted(vpcCNIConfigMap)
	})
	It("should initialize a windows node and schedule workloads", func() {
		// choose a EKS-based windows image
		out, err := env.SSMAPI.GetParameter(&ssm.GetParameterInput{
			Name: lo.ToPtr(fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", env.ExpectKubeServerVersion())),
		})
		Expect(err).ToNot(HaveOccurred())
		provider := awstest.AWSNodeTemplate(
			v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					AMIFamily:             lo.ToPtr("Custom"),
				},
				UserData: lo.ToPtr(env.ExpectWindowsUserData()),
				AMISelector: map[string]string{
					"aws-ids": lo.FromPtr(out.Parameter.Value),
				},
			},
		)
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpIn, Values: []string{string(v1.Windows)}},
				{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists},
				{Key: v1alpha1.LabelInstanceFamily, Operator: v1.NodeSelectorOpIn, Values: []string{
					"c5", "m5",
				}},
			},
		})
		numPods := 1
		deployment := test.Deployment(test.DeploymentOptions{Replicas: int32(numPods), PodOptions: test.PodOptions{
			Image: "mcr.microsoft.com/k8s/core/pause:1.2.0",
			NodeSelector: map[string]string{
				v1.LabelOSStable:   string(v1.Windows),
				v1.LabelArchStable: "amd64",
			},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1alpha1.ResourceAWSPrivateIPv4: resource.MustParse("1"),
				},
				Limits: v1.ResourceList{
					v1alpha1.ResourceAWSPrivateIPv4: resource.MustParse("1"),
				},
			},
		}})
		selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, deployment)

		// EventuallyExpectHealthyPodCount but with a longer timeout
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Monitor.RunningPodsCount(selector)).To(Equal(numPods))
		}, time.Minute*15).Should(Succeed())
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectCreatedNodesInitialized()
	})
})
