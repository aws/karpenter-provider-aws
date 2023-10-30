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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("CRD Hash", func() {
	It("should have Provisioner hash", func() {
		nodeTemplate := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: nodeTemplate.Name},
		})

		env.ExpectCreated(nodeTemplate, provisioner)

		Eventually(func(g Gomega) {
			var prov v1alpha5.Provisioner
			err := env.Client.Get(env, client.ObjectKeyFromObject(provisioner), &prov)
			g.Expect(err).ToNot(HaveOccurred())

			hash, found := prov.Annotations[v1alpha5.ProvisionerHashAnnotationKey]
			g.Expect(found).To(BeTrue())
			g.Expect(hash).To(Equal(provisioner.Hash()))
		})
	})
	It("should have AWSNodeTemplate hash", func() {
		nodeTemplate := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			},
		})
		env.ExpectCreated(nodeTemplate)

		Eventually(func(g Gomega) {
			var ant v1alpha1.AWSNodeTemplate
			err := env.Client.Get(env, client.ObjectKeyFromObject(nodeTemplate), &ant)
			g.Expect(err).ToNot(HaveOccurred())

			hash, found := ant.Annotations[v1alpha1.AnnotationNodeTemplateHash]
			g.Expect(found).To(BeTrue())
			g.Expect(hash).To(Equal(nodeTemplate.Hash()))
		})
	})
})
