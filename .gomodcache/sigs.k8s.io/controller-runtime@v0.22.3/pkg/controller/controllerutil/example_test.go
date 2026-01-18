/*
Copyright 2018 The Kubernetes Authors.

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

package controllerutil_test

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	log = logf.Log.WithName("controllerutil-examples")
)

// This example creates or updates an existing deployment.
func ExampleCreateOrUpdate() {
	// c is client.Client

	// Create or Update the deployment default/foo
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), c, deploy, func() error {
		// Deployment selector is immutable so we set this value only if
		// a new object is going to be created
		if deploy.ObjectMeta.CreationTimestamp.IsZero() {
			deploy.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			}
		}

		// update the Deployment pod template
		deploy.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "busybox",
						Image: "busybox",
					},
				},
			},
		}

		return nil
	})

	if err != nil {
		log.Error(err, "Deployment reconcile failed")
	} else {
		log.Info("Deployment successfully reconciled", "operation", op)
	}
}
