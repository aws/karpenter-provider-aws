package integration_test

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TTL Empty", func() {
	BeforeEach(func() { env.BeforeEach() })
	AfterEach(func() { env.AfterEach() })

	It("should terminate an empty node", func() {
		beforeNodes := env.Monitor.GetNodes()
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef:          &v1alpha5.ProviderRef{Name: provider.Name},
			TTLSecondsAfterEmpty: ptr.Int64(0),
		})

		const numPods = 1
		deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

		env.ExpectCreated(provider, provisioner, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), numPods)
		env.ExpectCreatedNodeCount("==", 1)

		createdNodes := env.GetCreatedNodes(beforeNodes, env.Monitor.GetNodes())

		persisted := deployment.DeepCopy()
		deployment.Spec.Replicas = ptr.Int32(0)
		Expect(env.Client.Patch(env, deployment, client.MergeFrom(persisted))).To(Succeed())

		env.ExpectNodesEventuallyDeleted(120*time.Second, createdNodes...)
	})
})

var _ = Describe("TTL Expired", func() {
	It("should terminate an expired node", func() {
		beforeNodes := env.Monitor.GetNodes()
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
		})

		const numPods = 1
		deployment := test.Deployment(test.DeploymentOptions{Replicas: numPods})

		env.ExpectCreated(provisioner, provider, deployment)
		env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), numPods)
		env.ExpectCreatedNodeCount("==", 1)

		createdNodes := env.GetCreatedNodes(beforeNodes, env.Monitor.GetNodes())

		persisted := provisioner.DeepCopy()
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(10)
		Expect(env.Client.Patch(env, provisioner, client.MergeFrom(persisted))).To(Succeed())

		env.ExpectNodesEventuallyDeleted(120*time.Second, createdNodes...)
	})
})
