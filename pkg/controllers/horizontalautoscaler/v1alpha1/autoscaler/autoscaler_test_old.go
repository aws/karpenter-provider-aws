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

// import (
// 	"reflect"
// 	"testing"

// 	"github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
// 	"github.com/ellistarn/karpenter/pkg/controllers/horizontalautoscaler/v1alpha1/algorithms"
// 	"github.com/ellistarn/karpenter/pkg/metrics/clients"
// 	v1 "k8s.io/api/autoscaling/v1"
// 	"k8s.io/apimachinery/pkg/runtime/schema"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// func TestFactoryFor(t *testing.T) {
// 	type args struct {
// 		resource *v1alpha1.HorizontalAutoscaler
// 	}
// 	tests := []struct {
// 		name string
// 		af   *Factory
// 		args args
// 		want Autoscaler
// 	}{{
// 		name: "creates resource",
// 		args: args{
// 			resource: &v1alpha1.HorizontalAutoscaler{},
// 		},
// 		af: &Factory{
// 			MetricsClientFactory: clients.Factory{},
// 			KubernetesClient:     client.DelegatingClient{},
// 		},
// 		want: Autoscaler{
// 			HorizontalAutoscaler: &v1alpha1.HorizontalAutoscaler{},
// 			algorithm:            &algorithms.Proportional{},
// 			kubernetesClient:     client.DelegatingClient{},
// 		},
// 	},
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := tt.af.For(tt.args.resource); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("Factory.For() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestAutoscalerReconcile(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		a       *Autoscaler
// 		wantErr bool
// 	}{
// 		{
// 			name: "Reconcile",
// 			a: &Autoscaler{
// 				HorizontalAutoscaler: &v1alpha1.HorizontalAutoscaler{
// 					Spec: v1alpha1.HorizontalAutoscalerSpec{
// 						Metrics: []v1alpha1.Metric{},
// 					},
// 				},
// 			},
// 		},
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := tt.a.Reconcile(); (err != nil) != tt.wantErr {
// 				t.Errorf("Autoscaler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestAutoscaler_getMetrics(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		a       *Autoscaler
// 		want    []algorithms.Metric
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := tt.a.getMetrics()
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("Autoscaler.getMetrics() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("Autoscaler.getMetrics() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestAutoscaler_getDesiredReplicas(t *testing.T) {
// 	type args struct {
// 		metrics  []algorithms.Metric
// 		replicas int32
// 	}
// 	tests := []struct {
// 		name string
// 		a    *Autoscaler
// 		args args
// 		want int32
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := tt.a.getDesiredReplicas(tt.args.metrics, tt.args.replicas); got != tt.want {
// 				t.Errorf("Autoscaler.getDesiredReplicas() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestAutoscaler_getScaleTarget(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		a       *Autoscaler
// 		want    *v1.Scale
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := tt.a.getScaleTarget()
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("Autoscaler.getScaleTarget() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("Autoscaler.getScaleTarget() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestAutoscaler_updateScaleTarget(t *testing.T) {
// 	type args struct {
// 		scaleTarget *v1.Scale
// 	}
// 	tests := []struct {
// 		name    string
// 		a       *Autoscaler
// 		args    args
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if err := tt.a.updateScaleTarget(tt.args.scaleTarget); (err != nil) != tt.wantErr {
// 				t.Errorf("Autoscaler.updateScaleTarget() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestAutoscaler_parseGroupResource(t *testing.T) {
// 	type args struct {
// 		scaleTargetRef v1alpha1.CrossVersionObjectReference
// 	}
// 	tests := []struct {
// 		name    string
// 		a       *Autoscaler
// 		args    args
// 		want    schema.GroupResource
// 		wantErr bool
// 	}{
// 		// TODO: Add test cases.
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := tt.a.parseGroupResource(tt.args.scaleTargetRef)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("Autoscaler.parseGroupResource() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("Autoscaler.parseGroupResource() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
