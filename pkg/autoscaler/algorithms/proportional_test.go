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

package algorithms

import (
	"testing"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/metrics"
)

// TestProportionalGetDesiredReplicas tests
func TestProportionalGetDesiredReplicas(t *testing.T) {
	type fields struct {
		Spec v1alpha1.HorizontalAutoscalerSpec
	}
	type args struct {
		metric   Metric
		replicas int32
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int32
	}{
		{
			name: "ValueMetricType normal case",
			args: args{
				metric: Metric{
					TargetType:  v1alpha1.ValueMetricType,
					TargetValue: 3,
					Metric: metrics.Metric{
						Value: 50,
					},
				},
				replicas: 8,
			},
			want: 134,
		},
		{
			name: "ValueMetricType does not scale from zero",
			args: args{
				metric: Metric{
					TargetType:  v1alpha1.ValueMetricType,
					TargetValue: 3,
					Metric: metrics.Metric{
						Value: 50,
					},
				},
				replicas: 0,
			},
			want: 0,
		},
		{
			name: "AverageValueMetricType normal case",
			args: args{
				metric: Metric{
					TargetType:  v1alpha1.AverageValueMetricType,
					TargetValue: 50,
					Metric: metrics.Metric{
						Value: 304,
					},
				},
				replicas: 1,
			},
			want: 7,
		},
		{
			name: "AverageValueMetricType scales to zero",
			args: args{
				metric: Metric{
					TargetType:  v1alpha1.AverageValueMetricType,
					TargetValue: 50,
					Metric: metrics.Metric{
						Value: 304,
					},
				},
				replicas: 0,
			},
			want: 7,
		},
		{
			name: "AverageUtilization normal case",
			args: args{
				metric: Metric{
					TargetType:  v1alpha1.UtilizationMetricType,
					TargetValue: 50,
					Metric: metrics.Metric{
						Value: .6,
					},
				},
				replicas: 2,
			},
			want: 3,
		},
		{
			name: "AverageUtilization does not scale to zero",
			args: args{
				metric: Metric{
					TargetType:  v1alpha1.UtilizationMetricType,
					TargetValue: 50,
					Metric: metrics.Metric{
						Value: .6,
					},
				},
				replicas: 0,
			},
			want: 0,
		},
		{
			name: "Unknown metric type returns replicas",
			args: args{
				metric: Metric{
					TargetType: "",
				},
				replicas: 50,
			},
			want: 50,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Proportional{
				Spec: tt.fields.Spec,
			}
			if got := a.GetDesiredReplicas(tt.args.metric, tt.args.replicas); got != tt.want {
				t.Errorf("Proportional.GetDesiredReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}
