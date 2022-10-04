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

package environment

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/onsi/gomega" //nolint:revive,stylecheck
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type pair[A, B any] struct {
	first  A
	second B
}

const TestLabelName = "testing.karpenter.sh/test-id"

func (env *Environment) ExpectCreated(objects ...client.Object) {
	for _, object := range objects {
		object.SetLabels(lo.Assign(object.GetLabels(), map[string]string{TestLabelName: env.ClusterName}))
		ExpectWithOffset(1, env.Client.Create(env, object)).To(Succeed())
	}
}

func (env *Environment) ExpectDeleted(objects ...client.Object) {
	for _, object := range objects {
		ExpectWithOffset(1, env.Client.Delete(env, object, &client.DeleteOptions{GracePeriodSeconds: ptr.Int64(0)})).To(Succeed())
	}
}

func (env *Environment) ExpectUpdate(objects ...client.Object) {
	for _, o := range objects {
		current := o.DeepCopyObject().(client.Object)
		ExpectWithOffset(1, env.Client.Get(env.Context, client.ObjectKeyFromObject(current), current)).To(Succeed())
		o.SetResourceVersion(current.GetResourceVersion())
		ExpectWithOffset(1, env.Client.Update(env.Context, o)).To(Succeed())
	}
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

func (env *Environment) EventuallyExpectKarpenterWithEnvVar(envVar v1.EnvVar) {
	EventuallyWithOffset(1, func(g Gomega) {
		labelMap := map[string]string{"app.kubernetes.io/instance": "karpenter"}
		listOptions := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()}
		podList, err := env.KubeClient.CoreV1().Pods("karpenter").List(env.Context, listOptions)
		g.Expect(err).ToNot(HaveOccurred())
		// we need all of the karpenter pods to have the new environment variable so that we don't return early
		// while some pods are still terminating
		for i := range podList.Items {
			g.Expect(podList.Items[i].Spec.Containers[0].Env).To(ContainElement(And(
				HaveField("Name", Equal(envVar.Name)),
				HaveField("Value", Equal(envVar.Value)),
			)))
			g.Expect(podList.Items[i].Status.Conditions).To(ContainElement(And(
				HaveField("Type", Equal(v1.PodReady)),
				HaveField("Status", Equal(v1.ConditionTrue)),
			)))
		}
	}).Should(Succeed())
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

func (env *Environment) eventuallyExpectScaleDown() {
	EventuallyWithOffset(1, func(g Gomega) {
		// expect the current node count to be what it was when the test started
		g.Expect(env.Monitor.NodeCount()).To(Equal(env.StartingNodeCount))
	}).Should(Succeed(), fmt.Sprintf("expected scale down to %d nodes, had %d", env.StartingNodeCount, env.Monitor.NodeCount()))
}

func (env *Environment) EventuallyExpectNotFound(objects ...client.Object) {
	for _, object := range objects {
		EventuallyWithOffset(1, func(g Gomega) {
			err := env.Client.Get(env, client.ObjectKeyFromObject(object), object)
			g.Expect(errors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed(), fmt.Sprintf("expcted %s to be deleted", client.ObjectKeyFromObject(object)))
	}
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

func (env *Environment) ExpectInstance(nodeName string) Assertion {
	return Expect(env.GetInstance(nodeName))
}

func (env *Environment) GetInstance(nodeName string) ec2.Instance {
	node := env.GetNode(nodeName)
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	ExpectWithOffset(1, len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instance, err := env.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, instance.Reservations).To(HaveLen(1))
	ExpectWithOffset(1, instance.Reservations[0].Instances).To(HaveLen(1))
	return *instance.Reservations[0].Instances[0]
}

func (env *Environment) GetVolume(volumeID *string) ec2.Volume {
	dvo, err := env.EC2API.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: []*string{volumeID}})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, len(dvo.Volumes)).To(Equal(1))
	return *dvo.Volumes[0]
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
