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

package readiness_test

import (
	"context"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/nodepool/readiness"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	controller    *readiness.Controller
	ctx           context.Context
	env           *test.Environment
	cloudProvider *fake.CloudProvider
	nodePool      *v1.NodePool
	nodeClass     *v1alpha1.TestNodeClass
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Counter")
}

var _ = BeforeSuite(func() {
	cloudProvider = fake.NewCloudProvider()
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	controller = readiness.NewController(env.Client, cloudProvider)
})
var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Readiness", func() {
	BeforeEach(func() {
		nodePool = test.NodePool()
		nodeClass = test.NodeClass(v1alpha1.TestNodeClass{
			ObjectMeta: metav1.ObjectMeta{Name: nodePool.Spec.Template.Spec.NodeClassRef.Name},
		})
		nodePool.Spec.Template.Spec.NodeClassRef.Group = object.GVK(nodeClass).Group
		nodePool.Spec.Template.Spec.NodeClassRef.Kind = object.GVK(nodeClass).Kind
	})
	It("should ignore NodePools which aren't managed by this instance of Karpenter", func() {
		nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
			Group: "karpenter.test.sh",
			Kind:  "UnmanagedNodeClass",
			Name:  "default",
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(status.ConditionReady).IsUnknown()).To(BeFalse())
	})
	It("should have status condition on nodePool as not ready when nodeClass does not exist", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
	})
	It("should have status condition on nodePool as ready if nodeClass is ready", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		_ = ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		Expect(nodePool.StatusConditions().IsTrue(status.ConditionReady)).To(BeTrue())
	})
	It("should have status condition on nodePool as not ready if nodeClass is not ready", func() {
		nodeClass.Status = v1alpha1.TestNodeClassStatus{
			Conditions: []status.Condition{
				{
					Type:               status.ConditionReady,
					Status:             metav1.ConditionFalse,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: metav1.Time{Time: time.Now()},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().IsTrue(status.ConditionReady)).To(BeFalse())
	})
	It("should have status condition on nodePool as not ready if nodeClass does not have status conditions", func() {
		nodeClass.Status = v1alpha1.TestNodeClassStatus{
			Conditions: []status.Condition{},
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().IsTrue(status.ConditionReady)).To(BeFalse())
	})
	It("should mark NodeClassReady status condition on nodePool as NotReady if nodeClass is terminating", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		ExpectDeletionTimestampSet(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		Expect(nodePool.StatusConditions().Get(status.ConditionReady).IsFalse()).To(BeTrue())
	})
})
