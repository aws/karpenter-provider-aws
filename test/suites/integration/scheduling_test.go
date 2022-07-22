package integration_test

import (
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)


var _ = Describe("Scheduling Conformance", func() {
	It("should provision a node for naked pods", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision a node for a deployment", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})

		const numPods = 50
		deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), numPods)
		env.ExpectCreatedNodeCount("<=", 2) // should probably all land on a single node, but at worst two depending on batching
	})
	It("should provision a node for a self-affinity deployment", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		// just two pods as they all need to land on the same node
		podLabels := map[string]string{"test": "self-affinity"}
		deployment := test.Deployment(test.DeploymentOptions{
			Replicas: 2,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				PodRequirements: []v1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: podLabels},
						TopologyKey:   v1.LabelHostname,
					},
				},
			},
		})

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), 2)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should provision three nodes for a zonal topology spread", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})

		// one pod per zone
		podLabels := map[string]string{"test": "zonal-spread"}
		deployment := test.Deployment(test.DeploymentOptions{
			Replicas: 3,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelTopologyZone,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector:     &metav1.LabelSelector{MatchLabels: podLabels},
					},
				},
			},
		})

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(podLabels), 3)
		env.ExpectCreatedNodeCount("==", 3)
	})
})
