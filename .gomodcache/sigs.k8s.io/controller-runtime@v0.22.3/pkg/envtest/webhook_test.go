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

package envtest

import (
	"context"
	"crypto/tls"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("Test", func() {
	Describe("Webhook", func() {
		It("should reject create request for webhook that rejects all requests", func(specCtx SpecContext) {
			m, err := manager.New(env.Config, manager.Options{
				WebhookServer: webhook.NewServer(webhook.Options{
					Port:    env.WebhookInstallOptions.LocalServingPort,
					Host:    env.WebhookInstallOptions.LocalServingHost,
					CertDir: env.WebhookInstallOptions.LocalServingCertDir,
					TLSOpts: []func(*tls.Config){func(config *tls.Config) {}},
				}),
			}) // we need manager here just to leverage manager.SetFields
			Expect(err).NotTo(HaveOccurred())
			server := m.GetWebhookServer()
			server.Register("/failing", &webhook.Admission{Handler: &rejectingValidator{}})

			ctx, cancel := context.WithCancel(specCtx)
			go func() {
				_ = server.Start(ctx)
			}()

			c, err := client.New(env.Config, client.Options{})
			Expect(err).NotTo(HaveOccurred())

			obj := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
						},
					},
				},
			}

			Eventually(func() bool {
				err = c.Create(ctx, obj)
				return err != nil && strings.HasSuffix(err.Error(), "Always denied") && apierrors.ReasonForError(err) == metav1.StatusReasonForbidden
			}, 1*time.Second).Should(BeTrue())

			cancel()
		})

		It("should load webhooks from directory", func() {
			installOptions := WebhookInstallOptions{
				Paths: []string{filepath.Join("testdata", "webhooks")},
			}
			err := parseWebhook(&installOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(installOptions.MutatingWebhooks).To(HaveLen(2))
			Expect(installOptions.ValidatingWebhooks).To(HaveLen(2))
		})

		It("should load webhooks from files", func() {
			installOptions := WebhookInstallOptions{
				Paths: []string{filepath.Join("testdata", "webhooks", "manifests.yaml")},
			}
			err := parseWebhook(&installOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(installOptions.MutatingWebhooks).To(HaveLen(2))
			Expect(installOptions.ValidatingWebhooks).To(HaveLen(2))
		})
	})
})

type rejectingValidator struct {
}

func (v *rejectingValidator) Handle(_ context.Context, _ admission.Request) admission.Response {
	return admission.Denied("Always denied")
}
