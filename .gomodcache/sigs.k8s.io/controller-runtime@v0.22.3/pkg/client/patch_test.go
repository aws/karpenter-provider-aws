/*
Copyright 2021 The Kubernetes Authors.

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

package client

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BenchmarkMergeFrom(b *testing.B) {
	cm1 := &corev1.ConfigMap{}
	cm1.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	cm1.ResourceVersion = "anything"

	cm2 := cm1.DeepCopy()
	cm2.Data = map[string]string{"key": "value"}

	sts1 := &appsv1.StatefulSet{}
	sts1.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("StatefulSet"))
	sts1.ResourceVersion = "somesuch"

	sts2 := sts1.DeepCopy()
	sts2.Spec.Template.Spec.Containers = []corev1.Container{{
		Resources: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1m"),
				corev1.ResourceMemory: resource.MustParse("1M"),
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{},
			},
		},
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				HTTPGet: &corev1.HTTPGetAction{},
			},
		},
		SecurityContext: &corev1.SecurityContext{},
	}}

	b.Run("NoOptions", func(b *testing.B) {
		cmPatch := MergeFrom(cm1)
		if _, err := cmPatch.Data(cm2); err != nil {
			b.Fatalf("expected no error, got %v", err)
		}

		stsPatch := MergeFrom(sts1)
		if _, err := stsPatch.Data(sts2); err != nil {
			b.Fatalf("expected no error, got %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cmPatch.Data(cm2)
			_, _ = stsPatch.Data(sts2)
		}
	})

	b.Run("WithOptimisticLock", func(b *testing.B) {
		cmPatch := MergeFromWithOptions(cm1, MergeFromWithOptimisticLock{})
		if _, err := cmPatch.Data(cm2); err != nil {
			b.Fatalf("expected no error, got %v", err)
		}

		stsPatch := MergeFromWithOptions(sts1, MergeFromWithOptimisticLock{})
		if _, err := stsPatch.Data(sts2); err != nil {
			b.Fatalf("expected no error, got %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cmPatch.Data(cm2)
			_, _ = stsPatch.Data(sts2)
		}
	})
}

var _ = Describe("MergeFrom", func() {
	It("should successfully create a patch for two large and similar in64s", func() {
		var largeInt64 int64 = 9223372036854775807
		var similarLargeInt64 int64 = 9223372036854775800
		j := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "test",
			},
			Spec: batchv1.JobSpec{
				ActiveDeadlineSeconds: &largeInt64,
			},
		}
		patch := MergeFrom(j.DeepCopy())

		j.Spec.ActiveDeadlineSeconds = &similarLargeInt64

		data, err := patch.Data(&j)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(Equal([]byte(`{"spec":{"activeDeadlineSeconds":9223372036854775800}}`)))
	})
})
