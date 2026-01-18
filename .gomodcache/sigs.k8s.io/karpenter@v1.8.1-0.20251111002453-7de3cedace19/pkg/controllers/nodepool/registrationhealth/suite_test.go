/*
Copyright The Kubernetes Authors.

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

package registrationhealth_test

import (
	"context"
	"testing"

	"sigs.k8s.io/karpenter/pkg/controllers/nodepool/registrationhealth"
	"sigs.k8s.io/karpenter/pkg/state/nodepoolhealth"

	"github.com/awslabs/operatorpkg/object"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	controller    *registrationhealth.Controller
	ctx           context.Context
	env           *test.Environment
	cloudProvider *fake.CloudProvider
	nodePool      *v1.NodePool
	nodeClass     *v1alpha1.TestNodeClass
	npState       *nodepoolhealth.State
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "RegistrationHealth")
}

var _ = BeforeSuite(func() {
	cloudProvider = fake.NewCloudProvider()
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	npState = nodepoolhealth.NewState()
	controller = registrationhealth.NewController(env.Client, cloudProvider, npState)
})
var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("RegistrationHealth", func() {
	BeforeEach(func() {
		nodePool = test.NodePool()
		nodeClass = test.NodeClass(v1alpha1.TestNodeClass{
			ObjectMeta: metav1.ObjectMeta{Name: nodePool.Spec.Template.Spec.NodeClassRef.Name},
		})
		nodePool.Spec.Template.Spec.NodeClassRef.Group = object.GVK(nodeClass).Group
		nodePool.Spec.Template.Spec.NodeClassRef.Kind = object.GVK(nodeClass).Kind
		_ = nodePool.StatusConditions().Clear(v1.ConditionTypeNodeRegistrationHealthy)
	})
	It("should ignore setting NodeRegistrationHealthy status condition on NodePools which aren't managed by this instance of Karpenter", func() {
		nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
			Group: "karpenter.test.sh",
			Kind:  "UnmanagedNodeClass",
			Name:  "default",
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy)).To(BeNil())
	})
	It("should not set NodeRegistrationHealthy status condition on nodePool when nodeClass does not exist", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy)).To(BeNil())
	})
	It("should set NodeRegistrationHealthy status condition on nodePool as Unknown and reset the NodeRegistrationHealthBuffer if the nodeClass observed generation doesn't match with that on nodePool", func() {
		nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, "unhealthy", "unhealthy")
		nodePool.Status.NodeClassObservedGeneration = int64(1)
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)

		nodeClass.Spec.Tags = map[string]string{"keyTag-1": "valueTag-1"}
		ExpectApplied(ctx, env.Client, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsUnknown()).To(BeTrue())
		Expect(nodePool.Status.NodeClassObservedGeneration).To(Equal(int64(2)))
		Expect(npState.Status(nodePool.UID)).To(BeEquivalentTo(nodepoolhealth.StatusUnknown))
	})
	It("should set NodeRegistrationHealthy status condition on nodePool as Unknown if the nodePool is updated", func() {
		nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, "unhealthy", "unhealthy")
		nodePool.Status.NodeClassObservedGeneration = int64(1)
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)

		nodePool.Spec.Limits = map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("14")}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsUnknown()).To(BeTrue())
		Expect(npState.Status(nodePool.UID)).To(BeEquivalentTo(nodepoolhealth.StatusUnknown))
	})
	It("should not set NodeRegistrationHealthy status condition on nodePool as Unknown if it is already set to true", func() {
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy)
		nodePool.Status.NodeClassObservedGeneration = int64(1)
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsUnknown()).To(BeFalse())
		Expect(npState.Status(nodePool.UID)).To(BeEquivalentTo(nodepoolhealth.StatusHealthy))
	})
	It("should not set NodeRegistrationHealthy status condition on nodePool as Unknown if it is already set to false", func() {
		nodePool.StatusConditions().SetFalse(v1.ConditionTypeNodeRegistrationHealthy, "unhealthy", "unhealthy")
		nodePool.Status.NodeClassObservedGeneration = int64(1)
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsUnknown()).To(BeFalse())
		Expect(npState.Status(nodePool.UID)).To(BeEquivalentTo(nodepoolhealth.StatusUnhealthy))
	})
})
