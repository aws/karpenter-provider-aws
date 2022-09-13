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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("KubeletConfiguration Overrides", func() {
	It("should schedule pods onto separate nodes when maxPods is set", func() {
		// Get the total number of daemonsets so that we see how many pods will be taken up
		// by daemonset overhead
		dsList := &appsv1.DaemonSetList{}
		Expect(env.Client.List(env.Context, dsList)).To(Succeed())
		dsCount := len(dsList.Items)

		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})

		// MaxPods needs to account for the daemonsets that will run on the nodes
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				MaxPods: ptr.Int32(1 + int32(dsCount)),
			},
		})

		pods := []*v1.Pod{test.Pod(), test.Pod(), test.Pod()}
		env.ExpectCreated(provisioner, provider)
		for _, pod := range pods {
			env.ExpectCreated(pod)
		}
		env.EventuallyExpectHealthy(pods...)
		env.ExpectCreatedNodeCount("==", 3)
	})
	It("should schedule pods onto separate nodes when podsPerCore is set", func() {
		// Get the total number of daemonsets so that we see how many pods will be taken up
		// by daemonset overhead
		dsList := &appsv1.DaemonSetList{}
		Expect(env.Client.List(env.Context, dsList)).To(Succeed())
		dsCount := len(dsList.Items)

		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		// PodsPerCore needs to account for the daemonsets that will run on the nodes
		// This will have 4 pods available on each node (2 taken by daemonset pods)
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				PodsPerCore: ptr.Int32(2 + int32(dsCount)),
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      awsv1alpha1.LabelInstanceCPU,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"1"},
				},
			},
		})

		pods := []*v1.Pod{test.Pod(), test.Pod(), test.Pod(), test.Pod()}
		env.ExpectCreated(provisioner, provider)
		for _, pod := range pods {
			env.ExpectCreated(pod)
		}
		env.EventuallyExpectHealthy(pods...)
		env.ExpectCreatedNodeCount("==", 2)
	})
	It("should ignore podsPerCore value when Bottlerocket is used", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			AMIFamily:             &awsv1alpha1.AMIFamilyBottlerocket,
		}})
		// All pods should schedule to a single node since we are ignoring podsPerCore value
		// This would normally schedule to 3 nodes if not using Bottlerocket
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				PodsPerCore: ptr.Int32(1),
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      awsv1alpha1.LabelInstanceCPU,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"1"},
				},
			},
		})

		pods := []*v1.Pod{test.Pod(), test.Pod(), test.Pod(), test.Pod(), test.Pod(), test.Pod()}
		env.ExpectCreated(provisioner, provider)
		for _, pod := range pods {
			env.ExpectCreated(pod)
		}
		env.EventuallyExpectHealthy(pods...)
		env.ExpectCreatedNodeCount("==", 1)
	})
})
