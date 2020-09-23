package expecations

import (
	"context"
	"fmt"
	"time"

	"github.com/ellistarn/karpenter/pkg/utils/log"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	APIServerPropagationTime  = 1 * time.Second
	ReconcilerPropagationTime = 10 * time.Second
	RequestInterval           = 1 * time.Second
)

type TestableObject interface {
	runtime.Object
	v1.Object
	IsHappy() bool
}

func ExpectEventuallyCreated(client client.Client, object TestableObject) {
	nn := types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}
	Expect(client.Create(context.Background(), object)).To(Succeed())
	Eventually(func() error {
		return client.Get(context.Background(), nn, object)
	}, APIServerPropagationTime, RequestInterval).Should(Succeed())
}

func ExpectEventuallyHappy(client client.Client, object TestableObject) {
	nn := types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}
	Eventually(func() bool {
		Expect(client.Get(context.Background(), nn, object)).To(Succeed())
		return object.IsHappy()
	}, ReconcilerPropagationTime, RequestInterval).Should(BeTrue(), func() string {
		return fmt.Sprintf("resource never became happy\n%s", log.Pretty(object))
	})
}

func ExpectEventuallyDeleted(client client.Client, object TestableObject) {
	nn := types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}
	Expect(client.Delete(context.Background(), object)).To(Succeed())
	Eventually(func() bool {
		return apierrors.IsNotFound(client.Get(context.Background(), nn, object))
	}, APIServerPropagationTime, RequestInterval).Should(BeTrue(), "resource was never deleted %s", nn)
}
