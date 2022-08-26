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
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TTL Empty", func() {
	BeforeEach(func() { env.BeforeEach() })
	AfterEach(func() { env.AfterEach() })

	It("should terminate an empty node", func() {
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

		persisted := deployment.DeepCopy()
		deployment.Spec.Replicas = ptr.Int32(0)
		Expect(env.Client.Patch(env, deployment, client.MergeFrom(persisted))).To(Succeed())

		for i := range env.Monitor.GetCreatedNodes() {
			node := env.Monitor.GetCreatedNodes()[i]
			env.EventuallyExpectNotFound(&node)
		}
	})
})

var _ = Describe("TTL Expired", func() {
	It("should terminate an expired node", func() {
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

		persistedDep := deployment.DeepCopy()
		deployment.Spec.Replicas = ptr.Int32(0)
		Expect(env.Client.Patch(env, deployment, client.MergeFrom(persistedDep))).To(Succeed())

		persistedProv := provisioner.DeepCopy()
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(10)
		Expect(env.Client.Patch(env, provisioner, client.MergeFrom(persistedProv))).To(Succeed())

		for i := range env.Monitor.GetCreatedNodes() {
			node := env.Monitor.GetCreatedNodes()[i]
			env.EventuallyExpectNotFound(&node)
		}
	})
})
