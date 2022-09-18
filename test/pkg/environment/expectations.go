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
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,stylecheck
	. "github.com/onsi/gomega"    //nolint:revive,stylecheck
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	nodeutils "github.com/aws/karpenter/pkg/utils/node"
	"github.com/aws/karpenter/pkg/utils/pod"
)

var (
	TestLabelName    = "testing.karpenter.sh/test-id"
	CleanableObjects = []client.Object{
		&v1.Pod{},
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&policyv1.PodDisruptionBudget{},
		&v1.PersistentVolumeClaim{},
		&v1.PersistentVolume{},
		&storagev1.StorageClass{},
		&v1alpha1.AWSNodeTemplate{},
		&v1alpha5.Provisioner{},
	}
)

const (
	NoWatch  = "NoWatch"
	NoEvents = "NoEvents"
)

// if set, logs additional information that may be useful in debugging an E2E test failure
var debugE2E = true
var testStartTime time.Time
var stop chan struct{}

// nolint:gocyclo
func (env *Environment) BeforeEach() {
	stop = make(chan struct{})
	testStartTime = time.Now()

	if debugE2E {
		fmt.Println("------- START BEFORE -------")
		defer fmt.Println("------- END BEFORE -------")
	}

	var nodes v1.NodeList
	Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
	if debugE2E {
		for i := range nodes.Items {
			fmt.Println(env.getNodeInformation(&nodes.Items[i]))
		}
	}
	for _, node := range nodes.Items {
		if len(node.Spec.Taints) == 0 && !node.Spec.Unschedulable {
			Fail(fmt.Sprintf("expected system pool node %s to be tainted", node.Name))
		}
	}

	var pods v1.PodList
	Expect(env.Client.List(env.Context, &pods)).To(Succeed())
	if debugE2E {
		for i := range pods.Items {
			fmt.Println(env.getPodInformation(&pods.Items[i]))
		}
	}
	for i := range pods.Items {
		Expect(pod.IsProvisionable(&pods.Items[i])).To(BeFalse(),
			fmt.Sprintf("expected to have no provisionable pods, found %s/%s", pods.Items[i].Namespace, pods.Items[i].Name))
		Expect(pods.Items[i].Namespace).ToNot(Equal("default"),
			fmt.Sprintf("expected no pods in the `default` namespace, found %s/%s", pods.Items[i].Namespace, pods.Items[i].Name))
	}
	// If the test is labeled as NoWatch, then the node/pod monitor will just list at the beginning
	// of the test rather than perform a watch during it
	if debugE2E && !lo.Contains(CurrentSpecReport().Labels(), NoWatch) {
		env.startNodeMonitor(stop)
		env.startPodMonitor(stop)
	}
	var provisioners v1alpha5.ProvisionerList
	Expect(env.Client.List(env.Context, &provisioners)).To(Succeed())
	Expect(provisioners.Items).To(HaveLen(0), "expected no provisioners to exist")
	env.Monitor.Reset()
	env.StartingNodeCount = env.Monitor.NodeCountAtReset()
}

func (env *Environment) getNodeInformation(n *v1.Node) string {
	pods, _ := nodeutils.GetNodePods(env, env.Client, n)
	return fmt.Sprintf("node %s ready=%s initialized=%s pods=%d taints=%v", n.Name, nodeutils.GetCondition(n, v1.NodeReady).Status, n.Labels[v1alpha5.LabelNodeInitialized], len(pods), n.Spec.Taints)
}

func (env *Environment) getPodInformation(p *v1.Pod) string {
	var containerInfo strings.Builder
	for _, c := range p.Status.ContainerStatuses {
		if containerInfo.Len() > 0 {
			fmt.Fprintf(&containerInfo, ", ")
		}
		fmt.Fprintf(&containerInfo, "%s restarts=%d", c.Name, c.RestartCount)
	}
	return fmt.Sprintf("pods %s/%s provisionable=%v phase=%s nodename=%s [%s]", p.Namespace, p.Name,
		pod.IsProvisionable(p), p.Status.Phase, p.Spec.NodeName, containerInfo.String())
}

// Partially copied from
// https://github.com/kubernetes/kubernetes/blob/04ee339c7a4d36b4037ce3635993e2a9e395ebf3/staging/src/k8s.io/kubectl/pkg/describe/describe.go#L4232
func getEventInformation(k types.NamespacedName, el *v1.EventList) string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("------- %s EVENTS -------\n", k))
	if len(el.Items) == 0 {
		return sb.String()
	}
	for _, e := range el.Items {
		source := e.Source.Component
		if source == "" {
			source = e.ReportingController
		}
		sb.WriteString(fmt.Sprintf("type=%s reason=%s from=%s message=%s\n",
			e.Type,
			e.Reason,
			source,
			strings.TrimSpace(e.Message)),
		)
	}
	return sb.String()
}

// startPodMonitor monitors all pods that are provisioned in a namespace outside kube-system
// and karpenter namespaces during a test
func (env *Environment) startPodMonitor(stop <-chan struct{}) {
	factory := informers.NewSharedInformerFactoryWithOptions(env.KubeClient, time.Second*30,
		informers.WithTweakListOptions(func(l *metav1.ListOptions) {
			l.FieldSelector = "metadata.namespace!=kube-system,metadata.namespace!=karpenter"
		}))
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Printf("[CREATED] %s\n", env.getPodInformation(obj.(*v1.Pod)))
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			if env.getPodInformation(oldObj.(*v1.Pod)) != env.getPodInformation(newObj.(*v1.Pod)) {
				fmt.Printf("[UPDATED] %s\n", env.getPodInformation(newObj.(*v1.Pod)))
			}
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Printf("[DELETED] %s\n", env.getPodInformation(obj.(*v1.Pod)))
		},
	})
	factory.Start(stop)
}

// startNodeMonitor monitors all nodes that are provisioned by any provisioners during a test
func (env *Environment) startNodeMonitor(stop <-chan struct{}) {
	factory := informers.NewSharedInformerFactoryWithOptions(env.KubeClient, time.Second*30,
		informers.WithTweakListOptions(func(l *metav1.ListOptions) { l.LabelSelector = v1alpha5.ProvisionerNameLabelKey }))
	podInformer := factory.Core().V1().Nodes().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*v1.Node)
			if _, ok := node.Labels[TestLabelName]; ok {
				fmt.Printf("[CREATED] %s\n", env.getNodeInformation(obj.(*v1.Node)))
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			if env.getNodeInformation(oldObj.(*v1.Node)) != env.getNodeInformation(newObj.(*v1.Node)) {
				fmt.Printf("[UPDATED] %s\n", env.getNodeInformation(newObj.(*v1.Node)))
			}
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Printf("[DELETED] %s\n", env.getNodeInformation(obj.(*v1.Node)))
		},
	})
	factory.Start(stop)
}

func (env *Environment) AfterEach() {
	if debugE2E {
		fmt.Println("------- START AFTER -------")
		defer fmt.Println("------- END AFTER -------")
	}

	namespaces := &v1.NamespaceList{}
	Expect(env.Client.List(env, namespaces)).To(Succeed())
	wg := sync.WaitGroup{}
	for _, object := range CleanableObjects {
		for _, namespace := range namespaces.Items {
			wg.Add(1)
			go func(object client.Object, namespace string) {
				defer GinkgoRecover()
				defer wg.Done()
				Expect(env.Client.DeleteAllOf(env, object,
					client.InNamespace(namespace),
					client.HasLabels([]string{TestLabelName}),
					client.PropagationPolicy(metav1.DeletePropagationForeground),
				)).To(Succeed())
			}(object, namespace.Name)
		}
	}
	wg.Wait()
	env.eventuallyExpectScaleDown()
	env.expectNoCrashes()
	close(stop) // close the pod/node monitor watch channel
	if debugE2E && !lo.Contains(CurrentSpecReport().Labels(), NoEvents) {
		env.dumpPodEvents(testStartTime)
	}
	env.printControllerLogs(&v1.PodLogOptions{Container: "controller"})
}

func (env *Environment) dumpPodEvents(testStartTime time.Time) {
	el := &v1.EventList{}
	ExpectWithOffset(1, env.Client.List(env, el)).To(Succeed())

	eventMap := map[types.NamespacedName]*v1.EventList{}

	filteredEvents := lo.Filter(el.Items, func(e v1.Event, _ int) bool {
		if !e.EventTime.IsZero() {
			if e.EventTime.BeforeTime(&metav1.Time{Time: testStartTime}) {
				return false
			}
		} else if e.FirstTimestamp.Before(&metav1.Time{Time: testStartTime}) {
			return false
		}
		if e.InvolvedObject.Kind != "Pod" {
			return false
		}
		if e.InvolvedObject.Namespace == "kube-system" || e.InvolvedObject.Namespace == "karpenter" {
			return false
		}
		return true
	})
	for i := range filteredEvents {
		elem := filteredEvents[i]
		objectKey := types.NamespacedName{Namespace: elem.InvolvedObject.Namespace, Name: elem.InvolvedObject.Name}
		if _, ok := eventMap[objectKey]; !ok {
			eventMap[objectKey] = &v1.EventList{}
		}
		eventMap[objectKey].Items = append(eventMap[objectKey].Items, elem)
	}
	for k, v := range eventMap {
		fmt.Print(getEventInformation(k, v))
	}
}

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
	ExpectWithOffset(1, env.Monitor.CreatedNodes()).To(BeNumerically(comparator, nodeCount),
		fmt.Sprintf("expected %d created nodes, had %d", nodeCount, env.Monitor.CreatedNodes()))
}

func (env *Environment) EventuallyExpectCreatedNodeCount(comparator string, nodeCount int) {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(env.Monitor.CreatedNodes()).To(BeNumerically(comparator, nodeCount),
			fmt.Sprintf("expected %d created nodes, had %d", nodeCount, env.Monitor.CreatedNodes()))
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
	Expect(len(providerIDSplit)).ToNot(Equal(0))
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instance, err := env.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: aws.StringSlice([]string{instanceID}),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(instance.Reservations).To(HaveLen(1))
	Expect(instance.Reservations[0].Instances).To(HaveLen(1))
	return *instance.Reservations[0].Instances[0]
}

func (env *Environment) GetVolume(volumeID *string) ec2.Volume {
	dvo, err := env.EC2API.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: []*string{volumeID}})
	Expect(err).ToNot(HaveOccurred())
	Expect(len(dvo.Volumes)).To(Equal(1))
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
	Expect(env.Client.List(env.Context, &events)).To(Succeed())
	for _, ev := range events.Items {
		if ev.InvolvedObject.Namespace == "karpenter" {
			if crashInfo.Len() > 0 {
				fmt.Fprintf(&crashInfo, ", ")
			}
			fmt.Fprintf(&crashInfo, "<%s/%s %s %s>", ev.InvolvedObject.Namespace, ev.InvolvedObject.Name, ev.Reason, ev.Message)
		}
	}

	Expect(crashed).To(BeFalse(), fmt.Sprintf("expected karpenter containers to not crash: %s", crashInfo.String()))
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
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.MinUtilization(resource)).To(BeNumerically(comparator, value))
	}).WithOffset(1).Should(Succeed())
}

func (env *Environment) EventuallyExpectAvgUtilization(resource v1.ResourceName, comparator string, value float64) {
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.AvgUtilization(resource)).To(BeNumerically(comparator, value))
	}, 10*time.Minute).WithOffset(1).Should(Succeed())
}
