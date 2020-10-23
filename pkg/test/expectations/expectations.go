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

	"github.com/ellistarn/karpenter/pkg/controllers"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	APIServerPropagationTime  = 1 * time.Second
	ReconcilerPropagationTime = 10 * time.Second
	RequestInterval           = 1 * time.Second
)

func ExpectCreated(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Expect(c.Create(context.Background(), object)).To(Succeed())
	}
}

func ExpectDeleted(c client.Client, objects ...client.Object) {
	for _, object := range objects {
		Expect(c.Delete(context.Background(), object)).To(Succeed())
	}
}

func ExpectEventuallyCreated(c client.Client, object client.Object) {
	nn := types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}
	Expect(c.Create(context.Background(), object)).To(Succeed())
	Eventually(func() error {
		return c.Get(context.Background(), nn, object)
	}, APIServerPropagationTime, RequestInterval).Should(Succeed())
}

func ExpectEventuallyHappy(c client.Client, resource controllers.Object) {
	nn := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	Eventually(func() bool {
		Expect(c.Get(context.Background(), nn, resource)).To(Succeed())
		return resource.StatusConditions().IsHappy()
	}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("resource never became happy\n%s", log.Pretty(resource))
	})
}

func ExpectEventuallyDeleted(c client.Client, resource client.Object) {
	nn := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	Expect(c.Delete(context.Background(), resource)).To(Succeed())
	Eventually(func() bool {
		return errors.IsNotFound(c.Get(context.Background(), nn, resource))
	}, APIServerPropagationTime, RequestInterval).Should(BeTrue(), "resource was never deleted %s", nn)
}
