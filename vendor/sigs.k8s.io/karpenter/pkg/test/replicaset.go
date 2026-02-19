/*
Copyright The Kubernetes Authors.

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
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicaSetOptions customizes a ReplicaSet.
type ReplicaSetOptions struct {
	metav1.ObjectMeta
	Selector   map[string]string
	PodOptions PodOptions
}

// ReplicaSet creates a test ReplicaSet with defaults that can be overridden by ReplicaSetOptions.
// Overrides are applied in order, with a last write wins semantic.
func ReplicaSet(overrides ...ReplicaSetOptions) *appsv1.ReplicaSet {
	options := ReplicaSetOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	if options.Name == "" {
		options.Name = strings.ToLower(randomdata.SillyName())
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	if options.Selector == nil {
		options.Selector = map[string]string{"app": options.Name}
	}
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: options.Name, Namespace: options.Namespace},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: options.Selector},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: options.Selector},
				Spec:       Pod(options.PodOptions).Spec,
			},
		},
	}
}
