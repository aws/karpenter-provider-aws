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
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

var _ = FDescribe("EC2NodeClass Migration Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
		nodePool = &karpv1.NodePool{}
	})
	It("should have migration key on for v1 resources", func() {
		env.ExpectCreated(nodeClass, nodePool)
		Expect(nodeClass.Annotations).To(HaveKey(karpv1.StoredVersionMigratedKey))
		Expect(nodePool.Annotations).To(HaveKey(karpv1.StoredVersionMigratedKey))
	})
	It("should update CRD status", func() {
		v1beta1NodeClass := test.BetaEC2NodeClass()
		env.ExpectCreated(v1beta1NodeClass)
		nodeClassCrd, _ := lo.Find(apis.CRDs, func(crd *apiextensionsv1.CustomResourceDefinition) bool {
			return crd.Name == object.GVK(nodeClass).Kind
		})
		crd := &apiextensionsv1.CustomResourceDefinition{}
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClassCrd), crd)).To(Succeed())
		}).WithTimeout(time.Second * 10).Should(Succeed())
	})
})
