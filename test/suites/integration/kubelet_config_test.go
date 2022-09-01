package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("KubeletConfiguration Overrides", func() {
	It("should schedule pods onto separate nodes when maxPods is set", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		// MaxPods needs to account for the daemonsets that will run on the nodes
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				MaxPods: ptr.Int32(3),
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
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		// PodsPerCore needs to account for the daemonsets that will run on the nodes
		// This will have 4 pods available on each node (2 taken by daemonset pods)
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				PodsPerCore: ptr.Int32(2),
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      awsv1alpha1.LabelInstanceCPU,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"2"},
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
})
