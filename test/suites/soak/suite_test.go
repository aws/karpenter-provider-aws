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

package soak_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	nodeutils "github.com/aws/karpenter-core/pkg/utils/node"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/debug"
	"github.com/aws/karpenter/test/pkg/environment/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	awssdk "github.com/aws/aws-sdk-go/aws"
)

var env *aws.Environment

func TestSoak(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
		SetDefaultEventuallyTimeout(time.Hour)
	})
	RunSpecs(t, "Soak")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Soak", func() {
	It("should ", Label(debug.NoWatch), Label(debug.NoEvents), func() {
		ctx, cancel := context.WithCancel(env.Context)
		defer cancel()

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeSpot},
				},
			},
			Consolidation: &v1alpha5.Consolidation{
				Enabled: lo.ToPtr(true),
			},
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
		})
		numPods := 0
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})

		dep.Spec.Template.Spec.Affinity = &v1.Affinity{
			PodAntiAffinity: &v1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
					{
						LabelSelector: dep.Spec.Selector,
						TopologyKey:   v1.LabelHostname,
					},
				},
			},
		}

		// Create a deployment with a single pod
		env.ExpectCreated(provider, provisioner, dep)
		startNodeCountMonitor(ctx, env.Client)
		time.Sleep(time.Second * 10)

		// Expect that we never get over a high number of nodes
		Consistently(func(g Gomega) {
			dep.Spec.Replicas = awssdk.Int32(int32(rand.Intn(20) + 1))
			env.ExpectUpdated(dep)
			time.Sleep(time.Minute * 1)
			dep.Spec.Replicas = awssdk.Int32(0)
			env.ExpectUpdated(dep)
			time.Sleep(time.Second * 30)
		}, time.Hour*12).Should(Succeed())
	})
})

func startNodeCountMonitor(ctx context.Context, kubeClient client.Client) {
	createdNodes := atomic.Int64{}
	deletedNodes := atomic.Int64{}

	factory := informers.NewSharedInformerFactoryWithOptions(env.KubeClient, time.Second*30,
		informers.WithTweakListOptions(func(l *metav1.ListOptions) { l.LabelSelector = v1alpha5.ProvisionerNameLabelKey }))
	nodeInformer := factory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(_ interface{}) {
			createdNodes.Add(1)
		},
		DeleteFunc: func(_ interface{}) {
			deletedNodes.Add(1)
		},
	})
	factory.Start(ctx.Done())
	go func() {
		for {
			list := &v1.NodeList{}
			if err := kubeClient.List(ctx, list, client.HasLabels{test.DiscoveryLabel}); err == nil {
				readyCount := lo.CountBy(list.Items, func(n v1.Node) bool {
					return nodeutils.GetCondition(&n, v1.NodeReady).Status == v1.ConditionTrue
				})
				fmt.Printf("[NODE COUNT] CURRENT: %d | READY: %d | CREATED: %d | DELETED: %d\n", len(list.Items), readyCount, createdNodes.Load(), deletedNodes.Load())
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second * 5):
			}
		}
	}()
}
