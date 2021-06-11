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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/log"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

func ExpectEventuallyDeleted(c client.Client, object client.Object) {
	Eventually(func() bool {
		return errors.IsNotFound(c.Get(context.Background(), types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}, object))
	}, APIServerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("expected %s/%s to be deleted, but it still exists", object.GetName(), object.GetNamespace())
	})
}

func ExpectCreated(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		nn := types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}
		Expect(c.Create(context.Background(), object)).To(Succeed())
		Eventually(func() error {
			return c.Get(context.Background(), nn, object)
		}, ReconcilerPropagationTime, RequestInterval).Should(Succeed())
	}
}

func ExpectCreatedWithStatus(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		ExpectCreated(c, object)
		Expect(c.Status().Update(context.Background(), object)).To(Succeed())
	}
}

func ExpectDeleted(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Expect(c.Delete(context.Background(), object)).To(Succeed())
	}
}

func ExpectEventuallyHappy(c client.Client, provisioner *v1alpha1.Provisioner) {
	nn := types.NamespacedName{Name: provisioner.GetName(), Namespace: provisioner.GetNamespace()}
	Eventually(func() bool {
		Expect(c.Get(context.Background(), nn, provisioner)).To(Succeed())
		return provisioner.StatusConditions().IsHappy()
	}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("resource never became happy\n%s", log.Pretty(provisioner))
	})
}

func ExpectEventuallyReconciled(c client.Client, provisioner *v1alpha1.Provisioner) {
	nn := types.NamespacedName{Name: provisioner.GetName(), Namespace: provisioner.GetNamespace()}
	Eventually(func() bool {
		Expect(c.Get(context.Background(), nn, provisioner)).To(Succeed())
		return !provisioner.StatusConditions().GetCondition(v1alpha1.Active).IsUnknown()
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
		ExpectDeleted(c, &node)
	}
	provisioners := v1alpha1.ProvisionerList{}
	Expect(c.List(ctx, &provisioners)).To(Succeed())
	for _, provisioner := range provisioners.Items {
		ExpectDeleted(c, &provisioner)
	}
}
