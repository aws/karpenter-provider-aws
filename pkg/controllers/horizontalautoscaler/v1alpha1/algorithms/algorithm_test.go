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
	"reflect"
	"testing"

	"github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
)

func TestFor(t *testing.T) {
	type args struct {
		spec v1alpha1.HorizontalAutoscalerSpec
	}
	tests := []struct {
		name string
		args args
		want Algorithm
	}{
		{
			name: "Proportional algorithm",
			args: args{v1alpha1.HorizontalAutoscalerSpec{}},
			want: &Proportional{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := For(tt.args.spec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("For() = %v, want %v", got, tt.want)
			}
		})
	}
}
