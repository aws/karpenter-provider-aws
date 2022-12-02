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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Emptiness", func() {
	It("should terminate an empty node", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
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

		nodes := env.Monitor.CreatedNodes()
		for i := range nodes {
			env.EventuallyExpectNotFound(nodes[i])
		}
	})
})
