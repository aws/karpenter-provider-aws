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
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
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

var _ = Describe("Windows", func() {
	BeforeEach(func() {
		env.ExpectCreatedOrUpdated(vpcCNIConfigMap)
	})
	AfterEach(func() {
		env.ExpectDeleted(vpcCNIConfigMap)
	})
	It("should initialize a windows node and schedule workloads", func() {
		// choose a EKS-based windows image
		windowsAMI := env.GetCustomAMI("/aws/service/ami-windows-latest/Windows_Server-2019-English-Core-EKS_Optimized-%s/image_id", 1)
		rawContent, err := os.ReadFile("testdata/windows_userdata_input.ps1")
		Expect(err).To(Succeed())
		provider := awstest.AWSNodeTemplate(
			v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					AMIFamily:             lo.ToPtr("Custom"),
				},
				AMISelector: map[string]string{
					"aws-ids": windowsAMI,
				},
			},
		)
		provider.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), settings.FromContext(env.Context).ClusterName,
			settings.FromContext(env.Context).ClusterEndpoint, env.ExpectCABundle()))
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpIn, Values: []string{string(v1.Windows)}},
				{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists},
			},
		})
		numPods := 1
		deployment := test.Deployment(test.DeploymentOptions{Replicas: int32(numPods), PodOptions: test.PodOptions{
			Image: "mcr.microsoft.com/k8s/core/pause:1.2.0",
			NodeSelector: map[string]string{
				v1.LabelOSStable:   string(v1.Windows),
				v1.LabelArchStable: "amd64",
			},
		}})
		selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

		env.ExpectCreated(provider, provisioner, deployment)
		// Wait for pods to go ready but with a longer timeout
		Eventually(func(g Gomega) {
			g.Expect(env.Monitor.RunningPodsCount(selector)).To(Equal(numPods))
		}).WithTimeout(15 * time.Minute).Should(Succeed())
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectCreatedNodesInitialized()
	})
})
