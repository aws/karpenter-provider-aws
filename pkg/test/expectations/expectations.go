package expecations

import (
	"context"
	"fmt"
	"time"

	"github.com/ellistarn/karpenter/pkg/controllers"
	"github.com/ellistarn/karpenter/pkg/test"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	APIServerPropagationTime  = 1 * time.Second
	ReconcilerPropagationTime = 10 * time.Second
	RequestInterval           = 1 * time.Second
)

func ExpectCreated(client client.Client, objects ...test.Resource) {
	for _, object := range objects {
		Expect(client.Create(context.Background(), object)).To(Succeed())
	}
}

func ExpectDeleted(client client.Client, objects ...test.Resource) {
	for _, object := range objects {
		Expect(client.Delete(context.Background(), object)).To(Succeed())
	}
}

func ExpectEventuallyCreated(client client.Client, object test.Resource) {
	nn := types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}
	Expect(client.Create(context.Background(), object)).To(Succeed())
	Eventually(func() error {
		return client.Get(context.Background(), nn, object)
	}, APIServerPropagationTime, RequestInterval).Should(Succeed())
}

func ExpectEventuallyHappy(client client.Client, resource controllers.Resource) {
	nn := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	Eventually(func() bool {
		Expect(client.Get(context.Background(), nn, resource)).To(Succeed())
		return resource.StatusConditions().IsHappy()
	}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("resource never became happy\n%s", log.Pretty(resource))
	})
}

func ExpectEventuallyDeleted(client client.Client, resource test.Resource) {
	nn := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
	Expect(client.Delete(context.Background(), resource)).To(Succeed())
	Eventually(func() bool {
		return apierrors.IsNotFound(client.Get(context.Background(), nn, resource))
	}, APIServerPropagationTime, RequestInterval).Should(BeTrue(), "resource was never deleted %s", nn)
}
