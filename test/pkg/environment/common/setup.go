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

package common

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pod"
	"github.com/aws/karpenter/test/pkg/debug"
)

var (
	CleanableObjects = []functional.Pair[client.Object, client.ObjectList]{
		{First: &v1.Pod{}, Second: &v1.PodList{}},
		{First: &appsv1.Deployment{}, Second: &appsv1.DeploymentList{}},
		{First: &appsv1.DaemonSet{}, Second: &appsv1.DaemonSetList{}},
		{First: &policyv1.PodDisruptionBudget{}, Second: &policyv1.PodDisruptionBudgetList{}},
		{First: &v1.PersistentVolumeClaim{}, Second: &v1.PersistentVolumeClaimList{}},
		{First: &v1.PersistentVolume{}, Second: &v1.PersistentVolumeList{}},
		{First: &storagev1.StorageClass{}, Second: &storagev1.StorageClassList{}},
		{First: &v1alpha5.Provisioner{}, Second: &v1alpha5.ProvisionerList{}},
		{First: &v1.LimitRange{}, Second: &v1.LimitRangeList{}},
		{First: &schedulingv1.PriorityClass{}, Second: &schedulingv1.PriorityClassList{}},
	}
	ForceCleanableObjects = []functional.Pair[client.Object, client.ObjectList]{
		{First: &v1.Node{}, Second: &v1.NodeList{}},
	}
)

// nolint:gocyclo
func (env *Environment) BeforeEach(opts ...Option) {
	options := ResolveOptions(opts)
	if !options.DisableDebug {
		fmt.Println("------- START BEFORE -------")
		defer fmt.Println("------- END BEFORE -------")

		// Run the debug logger BeforeEach() methods
		debug.BeforeEach(env.Context, env.Config, env.Client)
	}
	env.Context = injection.WithSettingsOrDie(env.Context, env.KubeClient, apis.Settings...)

	// Expect this cluster to be clean for test runs to execute successfully
	env.ExpectCleanCluster()

	var provisioners v1alpha5.ProvisionerList
	Expect(env.Client.List(env.Context, &provisioners)).To(Succeed())
	Expect(provisioners.Items).To(HaveLen(0), "expected no provisioners to exist")
	env.Monitor.Reset()
	env.StartingNodeCount = env.Monitor.NodeCountAtReset()
}

func (env *Environment) ExpectCleanCluster() {
	var nodes v1.NodeList
	Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
	for _, node := range nodes.Items {
		if len(node.Spec.Taints) == 0 && !node.Spec.Unschedulable {
			Fail(fmt.Sprintf("expected system pool node %s to be tainted", node.Name))
		}
	}
	var pods v1.PodList
	Expect(env.Client.List(env.Context, &pods)).To(Succeed())
	for i := range pods.Items {
		Expect(pod.IsProvisionable(&pods.Items[i])).To(BeFalse(),
			fmt.Sprintf("expected to have no provisionable pods, found %s/%s", pods.Items[i].Namespace, pods.Items[i].Name))
		Expect(pods.Items[i].Namespace).ToNot(Equal("default"),
			fmt.Sprintf("expected no pods in the `default` namespace, found %s/%s", pods.Items[i].Namespace, pods.Items[i].Name))
	}
}

func (env *Environment) Cleanup(opts ...Option) {
	options := ResolveOptions(opts)
	if !options.DisableDebug {
		fmt.Println("------- START CLEANUP -------")
		defer fmt.Println("------- END CLEANUP -------")

		// Run the debug logger AfterEach() methods
		debug.AfterEach(env.Context)
	}
	env.CleanupObjects(CleanableObjects)
	env.eventuallyExpectScaleDown()
	env.ExpectNoCrashes()
}

func (env *Environment) ForceCleanup(opts ...Option) {
	options := ResolveOptions(opts)
	if !options.DisableDebug {
		fmt.Println("------- START FORCE CLEANUP -------")
		defer fmt.Println("------- END FORCE CLEANUP -------")
	}

	// Delete all the nodes if they weren't deleted by the provisioner propagation
	env.CleanupObjects(ForceCleanableObjects)
}

func (env *Environment) AfterEach(opts ...Option) {
	options := ResolveOptions(opts)
	if !options.DisableDebug {
		fmt.Println("------- START AFTER -------")
		defer fmt.Println("------- END AFTER -------")
	}
	env.printControllerLogs(&v1.PodLogOptions{Container: "controller"})
}

func (env *Environment) CleanupObjects(cleanableObjects []functional.Pair[client.Object, client.ObjectList]) {
	namespaces := &v1.NamespaceList{}
	Expect(env.Client.List(env, namespaces)).To(Succeed())
	wg := sync.WaitGroup{}
	for _, p := range cleanableObjects {
		for _, namespace := range namespaces.Items {
			wg.Add(1)
			go func(obj client.Object, objList client.ObjectList, namespace string) {
				defer wg.Done()
				defer GinkgoRecover()
				Expect(env.Client.DeleteAllOf(env, obj,
					client.InNamespace(namespace),
					client.HasLabels([]string{test.DiscoveryLabel}),
					client.PropagationPolicy(metav1.DeletePropagationForeground),
				)).To(Succeed())
				Eventually(func(g Gomega) {
					stored := objList.DeepCopyObject().(client.ObjectList)
					g.Expect(env.Client.List(env, stored,
						client.InNamespace(namespace),
						client.HasLabels([]string{test.DiscoveryLabel}))).To(Succeed())
					items, err := meta.ExtractList(stored)
					g.Expect(err).To(Succeed())
					g.Expect(len(items)).To(BeZero())
				}).Should(Succeed())
			}(p.First, p.Second, namespace.Name)
		}
	}
	wg.Wait()
}
