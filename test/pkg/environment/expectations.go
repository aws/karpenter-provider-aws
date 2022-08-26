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
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
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

func (env *Environment) BeforeEach() {
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
	var provisioners v1alpha5.ProvisionerList
	Expect(env.Client.List(env.Context, &provisioners)).To(Succeed())
	Expect(provisioners.Items).To(HaveLen(0), "expected no provisioners to exist")
	env.Monitor.Reset()
}

func (env *Environment) AfterEach() {
	namespaces := &v1.NamespaceList{}
	Expect(env.Client.List(env, namespaces)).To(Succeed())
	wg := sync.WaitGroup{}
	for _, object := range CleanableObjects {
		for _, namespace := range namespaces.Items {
			wg.Add(1)
			go func(object client.Object, namespace string) {
				defer GinkgoRecover()
				Expect(env.Client.DeleteAllOf(env, object,
					client.InNamespace(namespace),
					client.HasLabels([]string{TestLabelName}),
					client.PropagationPolicy(metav1.DeletePropagationForeground),
				)).Should(Succeed())
				wg.Done()
			}(object, namespace.Name)
		}
	}
	wg.Wait()
	env.eventuallyExpectScaleDown()
	env.expectNoCrashes()
	env.printControllerLogs(&v1.PodLogOptions{Container: "controller"})
}

func (env *Environment) ExpectCreated(objects ...client.Object) {
	for _, object := range objects {
		object.SetLabels(lo.Assign(object.GetLabels(), map[string]string{TestLabelName: env.ClusterName}))
		Expect(env.Client.Create(env, object)).To(Succeed())
	}
}

func (env *Environment) ExpectDeleted(objects ...client.Object) {
	for _, object := range objects {
		Expect(env.Client.Delete(env, object, &client.DeleteOptions{GracePeriodSeconds: ptr.Int64(0)})).To(Succeed())
	}
}

func (env *Environment) ExpectUpdate(objects ...client.Object) {
	for _, o := range objects {
		current := o.DeepCopyObject().(client.Object)
		Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(current), current)).To(Succeed())
		o.SetResourceVersion(current.GetResourceVersion())
		Expect(env.Client.Update(env.Context, o)).To(Succeed())
	}
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

func (env *Environment) EventuallyExpectKarpenterWithEnvVar(envVar v1.EnvVar) {
	Eventually(func(g Gomega) {
		labelMap := map[string]string{"app.kubernetes.io/instance": "karpenter"}
		listOptions := metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labelMap).String()}
		podList, err := env.KubeClient.CoreV1().Pods("karpenter").List(env.Context, listOptions)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(podList.Items[0].Spec.Containers[0].Env).To(ContainElement(And(
			HaveField("Name", Equal(envVar.Name)),
			HaveField("Value", Equal(envVar.Value)),
		)))
		g.Expect(podList.Items[0].Status.Conditions).To(ContainElement(And(
			HaveField("Type", Equal(v1.PodReady)),
			HaveField("Status", Equal(v1.ConditionTrue)),
		)))
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectHealthyPodCount(selector labels.Selector, numPods int) {
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.RunningPods(selector)).To(Equal(numPods))
	}).Should(Succeed())
}

func (env *Environment) eventuallyExpectScaleDown() {
	Eventually(func(g Gomega) {
		// expect the current node count to be what it was when the test started
		g.Expect(env.Monitor.NodeCount()).To(Equal(env.Monitor.NodeCountAtReset()))
	}).Should(Succeed(), fmt.Sprintf("expected scale down to %d nodes, had %d", env.Monitor.NodeCountAtReset(), env.Monitor.NodeCount()))
}

func (env *Environment) EventuallyExpectNotFound(objects ...client.Object) {
	for _, object := range objects {
		Eventually(func(g Gomega) {
			err := env.Client.Get(env, client.ObjectKeyFromObject(object), object)
			g.Expect(errors.IsNotFound(err)).To(BeTrue())
		}).WithOffset(1).Should(Succeed(), fmt.Sprintf("expcted %s to be deleted", client.ObjectKeyFromObject(object)))
	}
}

func (env *Environment) ExpectCreatedNodeCount(comparator string, nodeCount int) {
	Expect(env.Monitor.CreatedNodes()).To(BeNumerically(comparator, nodeCount),
		fmt.Sprintf("expected %d created nodes, had %d", nodeCount, env.Monitor.CreatedNodes()))
}

func (env *Environment) GetNode(nodeName string) v1.Node {
	var node v1.Node
	Expect(env.Client.Get(env.Context, types.NamespacedName{Name: nodeName}, &node)).To(Succeed())
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
	for name, restartCount := range env.Monitor.RestartCount() {
		if restartCount > 0 {
			crashed = true
			env.printControllerLogs(&v1.PodLogOptions{Container: strings.Split(name, "/")[1], Previous: true})
		}
	}
	Expect(crashed).To(BeFalse(), "expected karpenter containers to not crash")
}

var (
	lastLogged = metav1.Now()
)

func (env *Environment) printControllerLogs(options *v1.PodLogOptions) {
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
	Expect(err).ToNot(HaveOccurred())
	log := new(bytes.Buffer)
	_, err = io.Copy(log, stream)
	Expect(err).ToNot(HaveOccurred())
	logging.FromContext(env.Context).Info(log)
}

func (env *Environment) EventuallyExpectMinUtilization(resource v1.ResourceName, comparator string, value float64) {
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.MinUtilization(resource)).To(BeNumerically(comparator, value))
	}).Should(Succeed())
}

func (env *Environment) EventuallyExpectAvgUtilization(resource v1.ResourceName, comparator string, value float64) {
	Eventually(func(g Gomega) {
		g.Expect(env.Monitor.AvgUtilization(resource)).To(BeNumerically(comparator, value))
	}, 10*time.Minute).Should(Succeed())
}
