package komega

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestDefaultGet(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	SetClient(fc)

	fetched := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
	}
	g.Eventually(Get(&fetched)).Should(Succeed())

	g.Expect(*fetched.Spec.Replicas).To(BeEquivalentTo(5))
}

func TestDefaultList(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	SetClient(fc)

	list := appsv1.DeploymentList{}
	g.Eventually(List(&list)).Should(Succeed())

	g.Expect(list.Items).To(HaveLen(1))
	depl := exampleDeployment()
	g.Expect(list.Items[0]).To(And(
		HaveField("ObjectMeta.Name", Equal(depl.ObjectMeta.Name)),
		HaveField("ObjectMeta.Namespace", Equal(depl.ObjectMeta.Namespace)),
	))
}

func TestDefaultUpdate(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	SetClient(fc)

	updateDeployment := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Eventually(Update(&updateDeployment, func() {
		updateDeployment.Annotations = map[string]string{"updated": "true"}
	})).Should(Succeed())

	fetched := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Expect(Object(&fetched)()).To(HaveField("ObjectMeta.Annotations", HaveKeyWithValue("updated", "true")))
}

func TestDefaultUpdateStatus(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	SetClient(fc)

	updateDeployment := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Eventually(UpdateStatus(&updateDeployment, func() {
		updateDeployment.Status.AvailableReplicas = 1
	})).Should(Succeed())

	fetched := appsv1.Deployment{
		ObjectMeta: exampleDeployment().ObjectMeta,
	}
	g.Expect(Object(&fetched)()).To(HaveField("Status.AvailableReplicas", BeEquivalentTo(1)))
}

func TestDefaultObject(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	SetClient(fc)

	fetched := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
	}
	g.Eventually(Object(&fetched)).Should(And(
		Not(BeNil()),
		HaveField("Spec.Replicas", Equal(ptr.To(int32(5)))),
	))
}

func TestDefaultObjectList(t *testing.T) {
	g := NewWithT(t)

	fc := createFakeClient()
	SetClient(fc)

	list := appsv1.DeploymentList{}
	g.Eventually(ObjectList(&list)).Should(And(
		Not(BeNil()),
		HaveField("Items", And(
			HaveLen(1),
			ContainElement(HaveField("Spec.Replicas", Equal(ptr.To(int32(5))))),
		)),
	))
}
