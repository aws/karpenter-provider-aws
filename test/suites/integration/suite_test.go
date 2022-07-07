package integration

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"testing"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var env *environment.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		var err error
		env, err = environment.NewEnvironment(t)
		Expect(err).ToNot(HaveOccurred())
	})
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeEach(func() {
	// Sets up the test monitor so we can count nodes per test as well as performing some checks to ensure any
	// existing nodes are tainted, there are no existing pods in the default namespace, etc.
	env.BeforeEach()
})
var _ = AfterEach(func() {
	env.AfterEach()
})

var _ = Describe("Sanity Checks", func() {
	It("should provision a node for a single pod", func() {
		provider := test.AWSNodeTemplate(test.AWSNodeTemplateOptions{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		// The 'CreatedNodeCount' doesn't count any nodes that are running when the test starts
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		// should have a new node created to support the pod
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectDeleted(pod)
		// all of the created nodes should be deleted
		env.EventuallyExpectScaleDown()
		// and neither the webhook or controller should have restarted during the test
		env.ExpectNoCrashes()
	})
	It("should provision for a deployment", func() {
		provider := test.AWSNodeTemplate(test.AWSNodeTemplateOptions{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})

		const numPods = 50
		deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

		selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		// should probably all land on a single node, but at worst two depending on batching
		env.ExpectCreatedNodeCount("<=", 2)
		env.ExpectDeleted(deployment)
		env.EventuallyExpectScaleDown()
		env.ExpectNoCrashes()
	})
	It("should provision a node for a self-afinity deployment", func() {
		provider := test.AWSNodeTemplate(test.AWSNodeTemplateOptions{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
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
		selector := labels.SelectorFromSet(podLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(selector, 2)
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectDeleted(deployment)
		env.EventuallyExpectScaleDown()
		env.ExpectNoCrashes()
	})
	It("should provision three nodes for a zonal topology spread", func() {
		provider := test.AWSNodeTemplate(test.AWSNodeTemplateOptions{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.Options.EnvironmentName},
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

		selector := labels.SelectorFromSet(podLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(selector, 3)
		env.ExpectCreatedNodeCount("==", 3)
		env.ExpectDeleted(deployment)
		env.EventuallyExpectScaleDown()
		env.ExpectNoCrashes()
	})
})
