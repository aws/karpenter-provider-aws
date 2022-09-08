package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Expiration", func() {
	It("should expire the node after the TTLSecondsUntilExpired is reached", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef:            &v1alpha5.ProviderRef{Name: provider.Name},
			TTLSecondsUntilExpired: ptr.Int64(30),
		})
		var numPods int32 = 3

		dep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})

		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, dep)

		// We don't care if the pod goes healthy, just if the node is expired
		env.EventuallyExpectCreatedNodeCount("==", 1)
		node := env.Monitor.GetCreatedNodes()[0]

		// Eventually expect the node to be gone
		env.EventuallyExpectNotFound(&node)
	})
	It("should replace expired node with a single node and schedule all pods", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})
		var numPods int32 = 5

		// We should setup a PDB that will only allow a minimum of 1 pod to be pending at a time
		minAvailable := intstr.FromInt(int(numPods) - 1)
		pdb := test.PodDisruptionBudget(test.PDBOptions{
			Labels: map[string]string{
				"app": "large-app",
			},
			MinAvailable: &minAvailable,
		})
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
			},
		})

		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(provisioner, provider, pdb, dep)

		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.ExpectCreatedNodeCount("==", 1)

		node := env.Monitor.GetCreatedNodes()[0]

		// Reset the monitor so that we can expect a single node to be spun up after expiration
		env.Monitor.Reset()

		// Set the TTLSecondsUntilExpired to get the node deleted
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(60)
		env.ExpectUpdate(provisioner)

		// Eventually the node deletion timestamp will be set
		Eventually(func(g Gomega) {
			n := &v1.Node{}
			g.Expect(env.Client.Get(env.Context, types.NamespacedName{Name: node.Name}, n)).Should(Succeed())
			g.Expect(n.DeletionTimestamp.IsZero()).Should(BeFalse())
		}).Should(Succeed())

		// Remove the TTLSecondsUntilExpired to make sure new node isn't deleted
		provisioner.Spec.TTLSecondsUntilExpired = nil
		env.ExpectUpdate(provisioner)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(&node)

		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.ExpectCreatedNodeCount("==", 1)
	})
})
