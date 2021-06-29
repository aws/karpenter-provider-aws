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

package expecations

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/utils/conditions"
	"github.com/awslabs/karpenter/pkg/utils/log"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	APIServerPropagationTime  = 1 * time.Second
	ReconcilerPropagationTime = 10 * time.Second
	RequestInterval           = 1 * time.Second
)

func ExpectPodExists(c client.Client, name string, namespace string) *v1.Pod {
	pod := &v1.Pod{}
	Expect(c.Get(context.Background(), client.ObjectKey{Name: name, Namespace: namespace}, pod)).To(Succeed())
	return pod
}

func ExpectNodeExists(c client.Client, name string) *v1.Node {
	node := &v1.Node{}
	Expect(c.Get(context.Background(), client.ObjectKey{Name: name}, node)).To(Succeed())
	return node
}

func ExpectNotFound(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Eventually(func() bool {
			return errors.IsNotFound(c.Get(context.Background(), types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}, object))
		}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
			return fmt.Sprintf("expected %s to be deleted, but it still exists", object.GetSelfLink())
		})
	}
}

func ExpectCreated(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Expect(c.Create(context.Background(), object)).To(Succeed())
	}
}

func ExpectCreatedWithStatus(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		// Preserve a copy of the status, which is overriden by create
		status := object.DeepCopyObject().(client.Object)
		ExpectCreated(c, object)
		Expect(c.Status().Update(context.Background(), status)).To(Succeed())
	}
}

func ExpectDeletedNode(c client.Client, n *v1.Node) {
	persisted := n.DeepCopy()
	n.Finalizers = []string{}
	Expect(c.Patch(context.Background(), n, client.MergeFrom(persisted))).To(Succeed())
	ExpectDeleted(c, n)
}

func ExpectDeleted(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Expect(c.Delete(context.Background(), object, &client.DeleteOptions{GracePeriodSeconds: ptr.Int64(0)})).To(Succeed())
	}
	for _, object := range objects {
		ExpectNotFound(c, object)
	}
}

func ExpectEventuallyHappy(c client.Client, provisioner *v1alpha2.Provisioner) {
	nn := types.NamespacedName{Name: provisioner.GetName(), Namespace: provisioner.GetNamespace()}
	Eventually(func() bool {
		Expect(c.Get(context.Background(), nn, provisioner)).To(Succeed())
		return provisioner.StatusConditions().IsHappy()
	}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("resource never became happy\n%s", log.Pretty(provisioner))
	})
}

func ExpectEventuallyReconciled(c client.Client, provisioner *v1alpha2.Provisioner) {
	nn := types.NamespacedName{Name: provisioner.GetName(), Namespace: provisioner.GetNamespace()}
	Eventually(func() bool {
		Expect(c.Get(context.Background(), nn, provisioner)).To(Succeed())
		return !provisioner.StatusConditions().GetCondition(conditions.Active).IsUnknown()
	}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("resources active condition was never updated\n%s", log.Pretty(provisioner))
	})
}

func ExpectCleanedUp(c client.Client) {
	ctx := context.Background()
	pods := v1.PodList{}
	Expect(c.List(ctx, &pods)).To(Succeed())
	for _, pod := range pods.Items {
		ExpectDeleted(c, &pod)
	}
	nodes := v1.NodeList{}
	Expect(c.List(ctx, &nodes)).To(Succeed())
	for _, node := range nodes.Items {
		ExpectDeletedNode(c, &node)
	}
	provisioners := v1alpha2.ProvisionerList{}
	Expect(c.List(ctx, &provisioners)).To(Succeed())
	for _, provisioner := range provisioners.Items {
		ExpectDeleted(c, &provisioner)
	}
}

func AttemptProvisioning(c client.Client, provisioner *v1alpha2.Provisioner, pod *v1.Pod) *v1.Pod {
	ExpectCreatedWithStatus(c, pod)
	ExpectCreated(c, provisioner)
	ExpectEventuallyReconciled(c, provisioner)
	return ExpectPodExists(c, pod.GetName(), pod.GetNamespace())
}
