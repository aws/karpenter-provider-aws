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

package test

import (
	"fmt"

	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodOptions customizes a Pod.
type ServiceOptions struct {
	ObjectMeta metav1.ObjectMeta
	ClusterIP  string
	Port       int32
	Conditions []metav1.Condition
}

// Pod creates a test pod with defaults that can be overridden by PodOptions.
// Overrides are applied in order, with a last write wins semantic.
func Service(overrides ...ServiceOptions) *v1.Service {
	options := ServiceOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge service options: %s", err))
		}
	}
	return &v1.Service{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1.ServiceSpec{
			ClusterIP: options.ClusterIP,
			Ports: []v1.ServicePort{
				{Port: options.Port},
			},
		},
		Status: v1.ServiceStatus{
			Conditions: options.Conditions,
		},
	}
}

func ServiceHaveClusterIP(namespace string, name string, clusterIP string) *v1.Service {
	return Service(ServiceOptions{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		ClusterIP:  clusterIP,
		Port:       80,
		Conditions: []metav1.Condition{{Status: metav1.ConditionTrue}},
	})
}
