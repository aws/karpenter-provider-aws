package komega

import (
	"testing"

	_ "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func exampleDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(5)),
		},
	}
}

func createFakeClient() client.Client {
	return fakeclient.NewClientBuilder().
		WithObjects(exampleDeployment()).
		Build()
}

func TestGet(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	k := New(fc)

	fetched := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
	}
	g.Eventually(k.Get(&fetched)).Should(Succeed())

	g.Expect(*fetched.Spec.Replicas).To(BeEquivalentTo(5))
}

func TestList(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	k := New(fc)

	list := appsv1.DeploymentList{}
	g.Eventually(k.List(&list)).Should(Succeed())

	g.Expect(list.Items).To(HaveLen(1))
	depl := exampleDeployment()
	g.Expect(list.Items[0]).To(And(
		HaveField("ObjectMeta.Name", Equal(depl.ObjectMeta.Name)),
		HaveField("ObjectMeta.Namespace", Equal(depl.ObjectMeta.Namespace)),
	))
}

func TestUpdate(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	k := New(fc)

	updateDeployment := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Eventually(k.Update(&updateDeployment, func() {
		updateDeployment.Annotations = map[string]string{"updated": "true"}
	})).Should(Succeed())

	fetched := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Expect(k.Object(&fetched)()).To(HaveField("ObjectMeta.Annotations", HaveKeyWithValue("updated", "true")))
}

func TestUpdateStatus(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	k := New(fc)

	updateDeployment := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Eventually(k.UpdateStatus(&updateDeployment, func() {
		updateDeployment.Status.AvailableReplicas = 1
	})).Should(Succeed())

	fetched := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Expect(k.Object(&fetched)()).To(HaveField("Status.AvailableReplicas", BeEquivalentTo(1)))
}

func TestObject(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	k := New(fc)

	fetched := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
	}
	g.Eventually(k.Object(&fetched)).Should(And(
		Not(BeNil()),
		HaveField("Spec.Replicas", Equal(ptr.To(int32(5)))),
	))
}

func TestObjectList(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	k := New(fc)

	list := appsv1.DeploymentList{}
	g.Eventually(k.ObjectList(&list)).Should(And(
		Not(BeNil()),
		HaveField("Items", And(
			HaveLen(1),
			ContainElement(HaveField("Spec.Replicas", Equal(ptr.To(int32(5))))),
		)),
	))
}
