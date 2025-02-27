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

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatefulSetOptions struct {
	metav1.ObjectMeta
	Labels     map[string]string
	Replicas   int32
	PodOptions PodOptions
}

func StatefulSet(overrides ...StatefulSetOptions) *appsv1.StatefulSet {
	options := StatefulSetOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge deployment options: %s", err))
		}
	}

	objectMeta := NamespacedObjectMeta(options.ObjectMeta)

	if options.PodOptions.Image == "" {
		options.PodOptions.Image = "public.ecr.aws/eks-distro/kubernetes/pause:3.2"
	}
	if options.PodOptions.Labels == nil {
		options.PodOptions.Labels = map[string]string{
			"app": objectMeta.Name,
		}
	}
	pod := Pod(options.PodOptions)
	return &appsv1.StatefulSet{
		ObjectMeta: objectMeta,
		Spec: appsv1.StatefulSetSpec{
			Replicas: lo.ToPtr(options.Replicas),
			Selector: &metav1.LabelSelector{MatchLabels: options.PodOptions.Labels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: ObjectMeta(options.PodOptions.ObjectMeta),
				Spec:       pod.Spec,
			},
		},
	}
}
