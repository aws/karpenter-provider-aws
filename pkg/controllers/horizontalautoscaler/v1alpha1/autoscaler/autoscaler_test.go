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

package autoscaler

import (
	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1/algorithms"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Factory", func() {
	type Args struct {
		resource *v1alpha1.HorizontalAutoscaler
	}
	type Test struct {
		af   *Factory
		args Args
		want Autoscaler
	}
	DescribeTable("For",
		func(test Test) {
			Ω(test.af.For(test.args.resource)).Should(Equal(test.want))
		},
		Entry("resource", Test{
			af: &Factory{
				MetricsClientFactory: clients.Factory{},
				KubernetesClient:     client.DelegatingClient{},
				Mapper:               &meta.DefaultRESTMapper{},
				ScaleNamespacer:      nil,
			},
			args: Args{
				resource: &v1alpha1.HorizontalAutoscaler{},
			},
			want: Autoscaler{
				HorizontalAutoscaler: &v1alpha1.HorizontalAutoscaler{},
				algorithm:            &algorithms.Proportional{},
				metricsClientFactory: clients.Factory{},
				kubernetesClient:     client.DelegatingClient{},
				mapper:               &meta.DefaultRESTMapper{},
			},
		}),
	)
})

var _ = Describe("Autoscaler", func() {
	scheme := runtime.NewScheme()
	apis.AddToScheme(scheme)

	type Args struct{}
	type Test struct {
		a       *Autoscaler
		args    Args
		wantErr error
	}
	DescribeTable("Reconcile",
		func(test Test) {
			if test.wantErr != nil {
				Ω(test.a.Reconcile()).Should(MatchError(test.wantErr))
			} else {
				Ω(test.a.Reconcile()).Should(Succeed())
			}
		},
		Entry("success", Test{
			a: &Autoscaler{
				algorithm:            &algorithms.Proportional{},
				kubernetesClient:     fake.NewFakeClientWithScheme(scheme),
				mapper:               meta.NewDefaultRESTMapper(scheme.PrioritizedVersionsAllGroups()),
				metricsClientFactory: clients.Factory{},
				HorizontalAutoscaler: &v1alpha1.HorizontalAutoscaler{
					Spec: v1alpha1.HorizontalAutoscalerSpec{
						ScaleTargetRef: v1alpha1.CrossVersionObjectReference{
							APIVersion: apis.GroupVersion.Identifier(),
							Kind:       "ScalableNodeGroup",
						},
					},
				},
			},
		}),
	)
})
