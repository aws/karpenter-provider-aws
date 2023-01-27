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
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
)

func (env *Environment) ExpectCreatedWithOffset(offset int, objects ...client.Object) {
	for _, object := range objects {
		object.SetLabels(lo.Assign(object.GetLabels(), map[string]string{
			test.DiscoveryLabel: "unspecified",
		}))
		ExpectWithOffset(offset+1, env.Client.Create(env, object)).To(Succeed())
	}
}

func (env *Environment) ExpectCreated(objects ...client.Object) {
	env.ExpectCreatedWithOffset(1, objects...)
}

func (env *Environment) ExpectDeletedWithOffset(offset int, objects ...client.Object) {
	for _, object := range objects {
		ExpectWithOffset(offset+1, env.Client.Delete(env, object, &client.DeleteOptions{GracePeriodSeconds: ptr.Int64(0)})).To(Succeed())
	}
}

func (env *Environment) ExpectDeleted(objects ...client.Object) {
	env.ExpectDeletedWithOffset(1, objects...)
}

func (env *Environment) ExpectUpdatedWithOffset(offset int, objects ...client.Object) {
	for _, o := range objects {
		current := o.DeepCopyObject().(client.Object)
		ExpectWithOffset(offset+1, env.Client.Get(env.Context, client.ObjectKeyFromObject(current), current)).To(Succeed())
		o.SetResourceVersion(current.GetResourceVersion())
		ExpectWithOffset(offset+1, env.Client.Update(env.Context, o)).To(Succeed())
	}
}

func (env *Environment) ExpectUpdated(objects ...client.Object) {
	env.ExpectUpdatedWithOffset(1, objects...)
}

func (env *Environment) ExpectCreatedOrUpdated(objects ...client.Object) {
	for _, o := range objects {
		current := o.DeepCopyObject().(client.Object)
		err := env.Client.Get(env, client.ObjectKeyFromObject(current), current)
		if err != nil {
			if errors.IsNotFound(err) {
				env.ExpectCreatedWithOffset(1, o)
			} else {
				Fail(fmt.Sprintf("Getting object %s, %v", client.ObjectKeyFromObject(o), err))
			}
		} else {
			env.ExpectUpdatedWithOffset(1, o)
		}
	}
}

func (env *Environment) ExpectSettings() *v1.ConfigMap {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karpenter-global-settings",
			Namespace: "karpenter",
		},
	}
	err := env.Client.Get(env, client.ObjectKeyFromObject(cm), cm)
	Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
	return cm
}

func (env *Environment) ExpectSettingsOverridden(data ...map[string]string) {
	cm := env.ExpectSettings()
	stored := cm.DeepCopy()
	cm.Data = lo.Assign(append([]map[string]string{cm.Data}, data...)...)

	// If the data hasn't changed, we can just return and not update anything
	if equality.Semantic.DeepEqual(stored, cm) {
		return
	}
	// Update the configMap to update the settings
	env.ExpectCreatedOrUpdated(cm)

	// Get the karpenter pods and delete them to restart the containers
	env.ExpectKarpenterPodsDeletedWithOffset(1)
	env.EventuallyExpectKarpenterPodsHealthyWithOffset(1)
}

func (env *Environment) ExpectFound(obj client.Object) {
	ExpectWithOffset(1, env.Client.Get(env, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
}

func (env *Environment) EventuallyExpectHealthy(pods ...*v1.Pod) {
	for _, pod := range pods {
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			g.Expect(pod.Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(v1.PodReady)),
				HaveField("Status", Equal(v1.ConditionTrue)),
			)))
		}).Should(Succeed())
	}
}

func (env *Environment) ExpectKarpenterPodsDeletedWithOffset(offset int) {
	podList := &v1.PodList{}
	ExpectWithOffset(offset+1, env.Client.List(env.Context, podList, client.MatchingLabels{
		"app.kubernetes.io/instance": "karpenter",
	})).To(Succeed())
	env.ExpectDeletedWithOffset(offset+1, lo.Map(podList.Items, func(p v1.Pod, _ int) client.Object {
		return &p
	})...)
	env.EventuallyExpectNotFoundWithOffset(1, lo.Map(podList.Items, func(p v1.Pod, _ int) client.Object {
		return &p
	})...)
}

func (env *Environment) EventuallyExpectKarpenterPodsHealthyWithOffset(offset int) {
	EventuallyWithOffset(offset+1, func(g Gomega) {
		podList := &v1.PodList{}
		g.Expect(env.Client.List(env.Context, podList, client.MatchingLabels{
			"app.kubernetes.io/instance": "karpenter",
		})).To(Succeed())
		for _, pod := range podList.Items {
			g.Expect(pod.Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(v1.PodReady)),
				HaveField("Status", Equal(v1.ConditionTrue)),
			)))
		}
	}).Should(Succeed())

	// We add this delay in here since we currently don't have the liveness/readiness probe working on the webhook
	// which means there's a bit of time after the pods go ready that the webhook isn't actually ready to receive traffic yet
	time.Sleep(time.Second * 5)
}

func (env *Environment) EventuallyExpectHealthyPodCount(selector labels.Selector, numPods int) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(env.Monitor.RunningPodsCount(selector)).To(Equal(numPods))
	}).Should(Succeed())
}

func (env *Environment) ExpectUniqueNodeNames(selector labels.Selector, uniqueNames int) {
	pods := env.Monitor.RunningPods(selector)
	nodeNames := sets.NewString()
	for _, pod := range pods {
		nodeNames.Insert(pod.Spec.NodeName)
	}
	ExpectWithOffset(1, len(nodeNames)).To(BeNumerically("==", uniqueNames))
}

func (env *Environment) EventuallyExpectCreatedNodesInitialized() {
	EventuallyWithOffset(1, func(g Gomega) {
		nodes := env.Monitor.CreatedNodes()
		nodeNames := sets.NewString(lo.Map(nodes, func(n *v1.Node, _ int) string { return n.Name })...)
		initializedNodeNames := sets.NewString(lo.FilterMap(nodes, func(n *v1.Node, _ int) (string, bool) {
			_, ok := n.Labels[v1alpha5.LabelNodeInitialized]
			return n.Name, ok
		})...)
		g.Expect(nodeNames.Equal(initializedNodeNames)).To(BeTrue())
	}).Should(Succeed())
}

func (env *Environment) eventuallyExpectScaleDown() {
	EventuallyWithOffset(1, func(g Gomega) {
		// expect the current node count to be what it was when the test started
		g.Expect(env.Monitor.NodeCount()).To(Equal(env.StartingNodeCount))
	}).Should(Succeed(), fmt.Sprintf("expected scale down to %d nodes, had %d", env.StartingNodeCount, env.Monitor.NodeCount()))
}

func (env *Environment) EventuallyExpectNotFound(objects ...client.Object) {
	env.EventuallyExpectNotFoundWithOffset(1, objects...)
}

func (env *Environment) EventuallyExpectNotFoundWithOffset(offset int, objects ...client.Object) {
	env.EventuallyExpectNotFoundAssertionWithOffset(offset+1, objects...).Should(Succeed())
}

func (env *Environment) EventuallyExpectNotFoundAssertion(objects ...client.Object) AsyncAssertion {
	return env.EventuallyExpectNotFoundAssertionWithOffset(1, objects...)
}

func (env *Environment) EventuallyExpectNotFoundAssertionWithOffset(offset int, objects ...client.Object) AsyncAssertion {
	return EventuallyWithOffset(offset, func(g Gomega) {
		for _, object := range objects {
			err := env.Client.Get(env, client.ObjectKeyFromObject(object), object)
			g.Expect(errors.IsNotFound(err)).To(BeTrue())
		}
	})
}

func (env *Environment) ExpectCreatedNodeCount(comparator string, nodeCount int) {
	ExpectWithOffset(1, env.Monitor.CreatedNodeCount()).To(BeNumerically(comparator, nodeCount),
		fmt.Sprintf("expected %d created nodes, had %d", nodeCount, env.Monitor.CreatedNodeCount()))
}

func (env *Environment) EventuallyExpectCreatedNodeCount(comparator string, nodeCount int) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(env.Monitor.CreatedNodeCount()).To(BeNumerically(comparator, nodeCount),
			fmt.Sprintf("expected %d created nodes, had %d", nodeCount, env.Monitor.CreatedNodeCount()))
	}).Should(Succeed())
}

func (env *Environment) GetNode(nodeName string) v1.Node {
	var node v1.Node
	ExpectWithOffset(1, env.Client.Get(env.Context, types.NamespacedName{Name: nodeName}, &node)).To(Succeed())
	return node
}

func (env *Environment) expectNoCrashes() {
	crashed := false
	var crashInfo strings.Builder
	for name, restartCount := range env.Monitor.RestartCount() {
		if restartCount > 0 {
			crashed = true
			env.printControllerLogs(&v1.PodLogOptions{Container: strings.Split(name, "/")[1], Previous: true})
			if crashInfo.Len() > 0 {
				fmt.Fprintf(&crashInfo, ", ")
			}
			fmt.Fprintf(&crashInfo, "%s restart count = %d", name, restartCount)
		}
	}

	// print any events in the karpenter namespace which may indicate liveness probes failing, etc.
	var events v1.EventList
	ExpectWithOffset(1, env.Client.List(env.Context, &events)).To(Succeed())
	for _, ev := range events.Items {
		if ev.InvolvedObject.Namespace == "karpenter" {
			if crashInfo.Len() > 0 {
				fmt.Fprintf(&crashInfo, ", ")
			}
			fmt.Fprintf(&crashInfo, "<%s/%s %s %s>", ev.InvolvedObject.Namespace, ev.InvolvedObject.Name, ev.Reason, ev.Message)
		}
	}

	ExpectWithOffset(1, crashed).To(BeFalse(), fmt.Sprintf("expected karpenter containers to not crash: %s", crashInfo.String()))
}

var (
	lastLogged = metav1.Now()
)

func (env *Environment) printControllerLogs(options *v1.PodLogOptions) {
	fmt.Println("------- START CONTROLLER LOGS -------")
	if options.SinceTime == nil {
		options.SinceTime = lastLogged.DeepCopy()
		lastLogged = metav1.Now()
	}
	lease, err := env.KubeClient.CoordinationV1().Leases("karpenter").Get(env.Context, "karpenter-leader-election", metav1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	Expect(lease.Spec.HolderIdentity).ToNot(BeNil(), "lease has no holder")
	nameid := strings.Split(*lease.Spec.HolderIdentity, "_")
	Expect(nameid).To(HaveLen(2), fmt.Sprintf("invalid lease HolderIdentity, %s", *lease.Spec.HolderIdentity))
	stream, err := env.KubeClient.CoreV1().Pods("karpenter").GetLogs(nameid[0], options).Stream(env.Context)
	if err != nil {
		logging.FromContext(env.Context).Errorf("fetching controller logs: %s", err)
		return
	}
	log := &bytes.Buffer{}
	_, err = io.Copy(log, stream)
	Expect(err).ToNot(HaveOccurred())
	logging.FromContext(env.Context).Info(log)
}

func (env *Environment) EventuallyExpectMinUtilization(resource v1.ResourceName, comparator string, value float64) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(env.Monitor.MinUtilization(resource)).To(BeNumerically(comparator, value))
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectAvgUtilization(resource v1.ResourceName, comparator string, value float64) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(env.Monitor.AvgUtilization(resource)).To(BeNumerically(comparator, value))
	}, 10*time.Minute).Should(Succeed())
}

func (env *Environment) ExpectDaemonSetEnvironmentVariableUpdated(obj client.ObjectKey, name, value string) {
	env.ExpectDaemonSetEnvironmentVariableUpdatedWithOffset(1, obj, name, value)
}

func (env *Environment) ExpectDaemonSetEnvironmentVariableUpdatedWithOffset(offset int, obj client.ObjectKey, name, value string) {
	ds := &appsv1.DaemonSet{}
	ExpectWithOffset(offset+1, env.Client.Get(env.Context, obj, ds)).To(Succeed())
	ExpectWithOffset(offset+1, len(ds.Spec.Template.Spec.Containers)).To(BeNumerically("==", 1))
	patch := client.MergeFrom(ds.DeepCopy())

	// If the value is found, update it. Else, create it
	found := false
	for i, v := range ds.Spec.Template.Spec.Containers[0].Env {
		if v.Name == name {
			ds.Spec.Template.Spec.Containers[0].Env[i].Value = value
			found = true
		}
	}
	if !found {
		ds.Spec.Template.Spec.Containers[0].Env = append(ds.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
	ExpectWithOffset(offset+1, env.Client.Patch(env.Context, ds, patch)).To(Succeed())
}

func (env *Environment) ExpectCABundle() string {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	transportConfig, err := env.Config.TransportConfig()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	logging.FromContext(env.Context).Debugf("Discovered caBundle, length %d", len(transportConfig.TLS.CAData))
	return base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)
}
