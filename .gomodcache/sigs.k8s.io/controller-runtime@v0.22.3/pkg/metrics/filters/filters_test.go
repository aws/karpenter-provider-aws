/*
Copyright 2023 The Kubernetes Authors.

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

package filters

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _ = Describe("manger.Manager", func() {
	Describe("Start", func() {
		Context("should start serving metrics with https and authn/authz", func() {
			var srv metricsserver.Server
			var defaultServer metricsDefaultServer
			var opts manager.Options
			var httpClient *http.Client

			BeforeEach(func() {
				srv = nil
				newMetricsServer := func(options metricsserver.Options, config *rest.Config, httpClient *http.Client) (metricsserver.Server, error) {
					var err error
					srv, err = metricsserver.NewServer(options, config, httpClient)
					if srv != nil {
						defaultServer = srv.(metricsDefaultServer)
					}
					return srv, err
				}
				opts = manager.Options{
					Metrics: metricsserver.Options{
						BindAddress:    ":0",
						SecureServing:  true,
						FilterProvider: WithAuthenticationAndAuthorization,
					},
				}
				v := reflect.ValueOf(&opts).Elem()
				newMetricsField := v.FieldByName("newMetricsServer")
				reflect.NewAt(newMetricsField.Type(), newMetricsField.Addr().UnsafePointer()).
					Elem().
					Set(reflect.ValueOf(newMetricsServer))
				httpClient = &http.Client{Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}}
			})

			It("should serve metrics in its registry", func(ctx SpecContext) {
				one := prometheus.NewCounter(prometheus.CounterOpts{
					Name: "test_one",
					Help: "test metric for testing",
				})
				one.Inc()
				err := metrics.Registry.Register(one)
				Expect(err).NotTo(HaveOccurred())

				m, err := manager.New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				// Setup service account with rights to "/metrics"
				token, cleanup, err := setupServiceAccountForURL(ctx, m.GetClient(), "/metrics")
				defer cleanup()
				Expect(err).ToNot(HaveOccurred())

				// GET /metrics with token.
				metricsEndpoint := fmt.Sprintf("https://%s/metrics", defaultServer.GetBindAddr())
				req, err := http.NewRequest("GET", metricsEndpoint, nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				// This is expected as the token has rights for /metrics.
				Expect(resp.StatusCode).To(Equal(200))

				data, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("%s\n%s\n%s\n",
					`# HELP test_one test metric for testing`,
					`# TYPE test_one counter`,
					`test_one 1`,
				))

				// Unregister will return false if the metric was never registered
				ok := metrics.Registry.Unregister(one)
				Expect(ok).To(BeTrue())
			})

			It("should serve extra endpoints", func(ctx SpecContext) {
				opts.Metrics.ExtraHandlers = map[string]http.Handler{
					"/debug": http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						_, _ = w.Write([]byte("Some debug info"))
					}),
				}
				m, err := manager.New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				// Setup service account with rights to "/debug"
				token, cleanup, err := setupServiceAccountForURL(ctx, m.GetClient(), "/debug")
				defer cleanup()
				Expect(err).ToNot(HaveOccurred())

				// GET /debug without token.
				endpoint := fmt.Sprintf("https://%s/debug", defaultServer.GetBindAddr())
				req, err := http.NewRequest("GET", endpoint, nil)
				Expect(err).NotTo(HaveOccurred())
				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				// This is expected as we didn't send a token.
				Expect(resp.StatusCode).To(Equal(401))
				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(ContainSubstring("Unauthorized"))

				// PUT /debug with token.
				req, err = http.NewRequest("PUT", endpoint, nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
				resp, err = httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				// This is expected as the token has rights for /debug.
				Expect(resp.StatusCode).To(Equal(200))
				body, err = io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal("Some debug info"))

				// GET /metrics with token (but token only has rights for /debug).
				metricsEndpoint := fmt.Sprintf("https://%s/metrics", defaultServer.GetBindAddr())
				req, err = http.NewRequest("GET", metricsEndpoint, nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
				resp, err = httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(403))
				body, err = io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				// Authorization denied is expected as the token only has rights for /debug not for /metrics.
				Expect(string(body)).To(ContainSubstring("Authorization denied for user system:serviceaccount:default:metrics-test"))
			})
		})
	})
})

type metricsDefaultServer interface {
	GetBindAddr() string
}

func setupServiceAccountForURL(ctx context.Context, c client.Client, path string) (string, func(), error) {
	createdObjects := []client.Object{}
	cleanup := func() {
		for _, obj := range createdObjects {
			_ = c.Delete(ctx, obj)
		}
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metrics-test",
			Namespace: metav1.NamespaceDefault,
		},
	}
	if err := c.Create(ctx, sa); err != nil {
		return "", cleanup, err
	}
	createdObjects = append(createdObjects, sa)

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metrics-test",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:           []string{"get", "put"},
				NonResourceURLs: []string{path},
			},
		},
	}
	if err := c.Create(ctx, cr); err != nil {
		return "", cleanup, err
	}
	createdObjects = append(createdObjects, cr)

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metrics-test",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "metrics-test",
				Namespace: metav1.NamespaceDefault,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "metrics-test",
		},
	}
	if err := c.Create(ctx, crb); err != nil {
		return "", cleanup, err
	}
	createdObjects = append(createdObjects, crb)

	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			ExpirationSeconds: ptr.To(int64(2 * 60 * 60)), // 2 hours.
		},
	}
	if err := c.SubResource("token").Create(ctx, sa, tokenRequest); err != nil {
		return "", cleanup, err
	}

	if tokenRequest.Status.Token == "" {
		return "", cleanup, errors.New("failed to get ServiceAccount token: token should not be empty")
	}

	return tokenRequest.Status.Token, cleanup, nil
}
