package environment

import (
	"sync"

	. "github.com/onsi/gomega" //nolint:revive,stylecheck
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

var (
	EnvironmentLabelName = "testing.karpenter.sh/environment-name"
	CleanableObjects     = []client.Object{
		&v1alpha5.Provisioner{},
		&v1.Pod{},
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&v1beta1.PodDisruptionBudget{},
		&v1.PersistentVolumeClaim{},
		&v1.PersistentVolume{},
		&storagev1.StorageClass{},
		&v1alpha1.AWSNodeTemplate{},
	}
)

func (env *Environment) ExpectCreated(objects ...client.Object) {
	for _, object := range objects {
		object.SetLabels(lo.Assign(object.GetLabels(), map[string]string{EnvironmentLabelName: env.Options.EnvironmentName}))
		Expect(env.Client.Create(env, object)).To(Succeed())
	}
}

func (env *Environment) ExpectCleaned() {
	namespaces := &v1.NamespaceList{}
	Expect(env.Client.List(env, namespaces)).To(Succeed())
	wg := sync.WaitGroup{}
	for _, object := range CleanableObjects {
		for _, namespace := range namespaces.Items {
			wg.Add(1)
			go func(object client.Object, namespace string) {
				Expect(env.Client.DeleteAllOf(env, object,
					client.InNamespace(namespace),
					client.MatchingLabels(map[string]string{EnvironmentLabelName: env.Options.EnvironmentName}),
				)).ToNot(HaveOccurred())
				wg.Done()
			}(object, namespace.Name)
		}
	}
	wg.Wait()
}

func (env *Environment) EventuallyExpectHealthy(pods ...*v1.Pod) {
	for _, pod := range pods {
		Eventually(func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(v1.PodReady)),
				HaveField("Status", Equal(v1.ConditionTrue)),
			)))
		}).Should(Succeed())
	}
}
