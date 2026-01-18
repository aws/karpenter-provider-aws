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

package manager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/goleak"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	intrec "sigs.k8s.io/controller-runtime/pkg/internal/recorder"
	"sigs.k8s.io/controller-runtime/pkg/leaderelection"
	fakeleaderelection "sigs.k8s.io/controller-runtime/pkg/leaderelection/fake"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ = Describe("manger.Manager", func() {
	Describe("New", func() {
		It("should return an error if there is no Config", func() {
			m, err := New(nil, Options{})
			Expect(m).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("must specify Config"))

		})

		It("should return an error if it can't create a RestMapper", func() {
			expected := fmt.Errorf("expected error: RestMapper")
			m, err := New(cfg, Options{
				MapperProvider: func(c *rest.Config, httpClient *http.Client) (meta.RESTMapper, error) { return nil, expected },
			})
			Expect(m).To(BeNil())
			Expect(err).To(Equal(expected))

		})

		It("should return an error it can't create a client.Client", func() {
			m, err := New(cfg, Options{
				NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
					return nil, errors.New("expected error")
				},
			})
			Expect(m).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
		})

		It("should return an error it can't create a cache.Cache", func() {
			m, err := New(cfg, Options{
				NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
					return nil, fmt.Errorf("expected error")
				},
			})
			Expect(m).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
		})

		It("should create a client defined in by the new client function", func() {
			m, err := New(cfg, Options{
				NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
					return nil, nil
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
			Expect(m.GetClient()).To(BeNil())
		})

		It("should return an error it can't create a recorder.Provider", func() {
			m, err := New(cfg, Options{
				newRecorderProvider: func(_ *rest.Config, _ *http.Client, _ *runtime.Scheme, _ logr.Logger, _ intrec.EventBroadcasterProducer) (*intrec.Provider, error) {
					return nil, fmt.Errorf("expected error")
				},
			})
			Expect(m).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected error"))
		})

		It("should lazily initialize a webhook server if needed", func() {
			By("creating a manager with options")
			m, err := New(cfg, Options{WebhookServer: webhook.NewServer(webhook.Options{Port: 9440, Host: "foo.com"})})
			Expect(err).NotTo(HaveOccurred())
			Expect(m).NotTo(BeNil())

			By("checking options are passed to the webhook server")
			svr := m.GetWebhookServer()
			Expect(svr).NotTo(BeNil())
			Expect(svr.(*webhook.DefaultServer).Options.Port).To(Equal(9440))
			Expect(svr.(*webhook.DefaultServer).Options.Host).To(Equal("foo.com"))
		})

		It("should not initialize a webhook server if Options.WebhookServer is set", func() {
			By("creating a manager with options")
			srv := webhook.NewServer(webhook.Options{Port: 9440})
			m, err := New(cfg, Options{WebhookServer: srv})
			Expect(err).NotTo(HaveOccurred())
			Expect(m).NotTo(BeNil())

			By("checking the server contains the Port set on the webhook server and not passed to Options")
			svr := m.GetWebhookServer()
			Expect(svr).NotTo(BeNil())
			Expect(svr).To(Equal(srv))
			Expect(svr.(*webhook.DefaultServer).Options.Port).To(Equal(9440))
		})

		It("should allow passing a custom webhook.Server implementation", func() {
			type customWebhook struct {
				webhook.Server
			}
			m, err := New(cfg, Options{WebhookServer: customWebhook{}})
			Expect(err).NotTo(HaveOccurred())
			Expect(m).NotTo(BeNil())

			svr := m.GetWebhookServer()
			Expect(svr).NotTo(BeNil())

			_, isCustomWebhook := svr.(customWebhook)
			Expect(isCustomWebhook).To(BeTrue())
		})

		Context("with leader election enabled", func() {
			It("should only cancel the leader election after all runnables are done", func(specCtx SpecContext) {
				m, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id-2",
					HealthProbeBindAddress:  "0",
					Metrics:                 metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:        "0",
				})
				Expect(err).ToNot(HaveOccurred())
				gvkcorev1 := schema.GroupVersionKind{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Kind: "ConfigMap"}
				gvkcoordinationv1 := schema.GroupVersionKind{Group: coordinationv1.SchemeGroupVersion.Group, Version: coordinationv1.SchemeGroupVersion.Version, Kind: "Lease"}
				Expect(m.GetScheme().Recognizes(gvkcorev1)).To(BeTrue())
				Expect(m.GetScheme().Recognizes(gvkcoordinationv1)).To(BeTrue())
				runnableDone := make(chan struct{})
				slowRunnable := RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					time.Sleep(100 * time.Millisecond)
					close(runnableDone)
					return nil
				})
				Expect(m.Add(slowRunnable)).To(Succeed())

				cm := m.(*controllerManager)
				cm.gracefulShutdownTimeout = time.Second
				leaderElectionDone := make(chan struct{})
				cm.onStoppedLeading = func() {
					close(leaderElectionDone)
				}

				ctx, cancel := context.WithCancel(specCtx)
				mgrDone := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).To(Succeed())
					close(mgrDone)
				}()
				<-cm.Elected()
				cancel()
				select {
				case <-leaderElectionDone:
					Expect(errors.New("leader election was cancelled before runnables were done")).ToNot(HaveOccurred())
				case <-runnableDone:
					// Success
				}
				// Don't leak routines
				<-mgrDone

			})
			It("should disable gracefulShutdown when stopping to lead", func(ctx SpecContext) {
				m, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id-3",
					HealthProbeBindAddress:  "0",
					Metrics:                 metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:        "0",
				})
				Expect(err).ToNot(HaveOccurred())

				mgrDone := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					err := m.Start(ctx)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("leader election lost"))
					close(mgrDone)
				}()
				cm := m.(*controllerManager)
				<-cm.elected

				cm.leaderElectionCancel()
				<-mgrDone

				Expect(cm.gracefulShutdownTimeout.Nanoseconds()).To(Equal(int64(0)))
			})

			It("should prevent leader election when shutting down a non-elected manager", func(specCtx SpecContext) {
				var rl resourcelock.Interface
				m1, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id",
					newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
						var err error
						rl, err = leaderelection.NewResourceLock(config, recorderProvider, options)
						return rl, err
					},
					HealthProbeBindAddress: "0",
					Metrics:                metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:       "0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m1).ToNot(BeNil())
				Expect(rl.Describe()).To(Equal("default/test-leader-election-id"))

				m1cm, ok := m1.(*controllerManager)
				Expect(ok).To(BeTrue())
				m1cm.onStoppedLeading = func() {}

				m2, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id",
					newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
						var err error
						rl, err = leaderelection.NewResourceLock(config, recorderProvider, options)
						return rl, err
					},
					HealthProbeBindAddress: "0",
					Metrics:                metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:       "0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m2).ToNot(BeNil())
				Expect(rl.Describe()).To(Equal("default/test-leader-election-id"))

				m1done := make(chan struct{})
				Expect(m1.Add(RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					close(m1done)
					return nil
				}))).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m1.Elected()).ShouldNot(BeClosed())
					Expect(m1.Start(specCtx)).NotTo(HaveOccurred())
				}()
				<-m1.Elected()
				<-m1done

				electionRunnable := &needElection{make(chan struct{})}

				Expect(m2.Add(electionRunnable)).To(Succeed())

				ctx2, cancel2 := context.WithCancel(specCtx)
				m2done := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(m2.Start(ctx2)).NotTo(HaveOccurred())
					close(m2done)
				}()
				Consistently(m2.Elected()).ShouldNot(Receive())

				go func() {
					defer GinkgoRecover()
					Consistently(electionRunnable.ch).ShouldNot(Receive())
				}()
				cancel2()
				<-m2done
			})

			It("should default RenewDeadline for leader election config", func() {
				var rl resourcelock.Interface
				m1, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id",
					newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
						if options.RenewDeadline != 10*time.Second {
							return nil, fmt.Errorf("expected RenewDeadline to be 10s, got %v", options.RenewDeadline)
						}
						var err error
						rl, err = leaderelection.NewResourceLock(config, recorderProvider, options)
						return rl, err
					},
					HealthProbeBindAddress: "0",
					Metrics:                metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:       "0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m1).ToNot(BeNil())
			})

			It("should default ID to controller-runtime if ID is not set", func(specCtx SpecContext) {
				var rl resourcelock.Interface
				m1, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id",
					newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
						var err error
						rl, err = leaderelection.NewResourceLock(config, recorderProvider, options)
						return rl, err
					},
					HealthProbeBindAddress: "0",
					Metrics:                metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:       "0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m1).ToNot(BeNil())
				Expect(rl.Describe()).To(Equal("default/test-leader-election-id"))

				m1cm, ok := m1.(*controllerManager)
				Expect(ok).To(BeTrue())
				m1cm.onStoppedLeading = func() {}

				m2, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionNamespace: "default",
					LeaderElectionID:        "test-leader-election-id",
					newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
						var err error
						rl, err = leaderelection.NewResourceLock(config, recorderProvider, options)
						return rl, err
					},
					HealthProbeBindAddress: "0",
					Metrics:                metricsserver.Options{BindAddress: "0"},
					PprofBindAddress:       "0",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m2).ToNot(BeNil())
				Expect(rl.Describe()).To(Equal("default/test-leader-election-id"))

				m2cm, ok := m2.(*controllerManager)
				Expect(ok).To(BeTrue())
				m2cm.onStoppedLeading = func() {}

				c1 := make(chan struct{})
				Expect(m1.Add(RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					close(c1)
					return nil
				}))).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m1.Elected()).ShouldNot(BeClosed())
					Expect(m1.Start(specCtx)).NotTo(HaveOccurred())
				}()
				<-m1.Elected()
				<-c1

				c2 := make(chan struct{})
				Expect(m2.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					close(c2)
					return nil
				}))).To(Succeed())

				ctx2, cancel := context.WithCancel(specCtx)
				m2done := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(m2.Start(ctx2)).NotTo(HaveOccurred())
					close(m2done)
				}()
				Consistently(m2.Elected()).ShouldNot(Receive())

				Consistently(c2).ShouldNot(Receive())
				cancel()
				<-m2done
			})

			It("should return an error if it can't create a ResourceLock", func() {
				m, err := New(cfg, Options{
					newResourceLock: func(_ *rest.Config, _ recorder.Provider, _ leaderelection.Options) (resourcelock.Interface, error) {
						return nil, fmt.Errorf("expected error")
					},
				})
				Expect(m).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("expected error")))
			})

			It("should return an error if namespace not set and not running in cluster", func() {
				m, err := New(cfg, Options{LeaderElection: true, LeaderElectionID: "controller-runtime"})
				Expect(m).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unable to find leader election namespace: not running in-cluster, please specify LeaderElectionNamespace"))
			})

			// We must keep this default until we are sure all controller-runtime users have upgraded from the original default
			// ConfigMap lock to a controller-runtime version that has this new default. Many users of controller-runtime skip
			// versions, so we should be extremely conservative here.
			It("should default to LeasesResourceLock", func() {
				m, err := New(cfg, Options{LeaderElection: true, LeaderElectionID: "controller-runtime", LeaderElectionNamespace: "my-ns"})
				Expect(m).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				cm, ok := m.(*controllerManager)
				Expect(ok).To(BeTrue())
				_, isLeaseLock := cm.resourceLock.(*resourcelock.LeaseLock)
				Expect(isLeaseLock).To(BeTrue())

			})
			It("should use the specified ResourceLock", func() {
				m, err := New(cfg, Options{
					LeaderElection:             true,
					LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
					LeaderElectionID:           "controller-runtime",
					LeaderElectionNamespace:    "my-ns",
				})
				Expect(m).ToNot(BeNil())
				Expect(err).ToNot(HaveOccurred())
				cm, ok := m.(*controllerManager)
				Expect(ok).To(BeTrue())
				_, isLeaseLock := cm.resourceLock.(*resourcelock.LeaseLock)
				Expect(isLeaseLock).To(BeTrue())
			})
			It("should release lease if ElectionReleaseOnCancel is true", func(specCtx SpecContext) {
				var rl resourcelock.Interface
				m, err := New(cfg, Options{
					LeaderElection:                true,
					LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
					LeaderElectionID:              "controller-runtime",
					LeaderElectionNamespace:       "my-ns",
					LeaderElectionReleaseOnCancel: true,
					newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
						var err error
						rl, err = fakeleaderelection.NewResourceLock(config, recorderProvider, options)
						return rl, err
					},
				})
				Expect(err).ToNot(HaveOccurred())

				ctx, cancel := context.WithCancel(specCtx)
				doneCh := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					defer close(doneCh)
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.(*controllerManager).elected
				cancel()
				<-doneCh

				record, _, err := rl.Get(specCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(record.HolderIdentity).To(BeEmpty())
			})
			It("should set the leaselocks's label field when LeaderElectionLabels is set", func() {
				labels := map[string]string{"my-key": "my-val"}
				m, err := New(cfg, Options{
					LeaderElection:             true,
					LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
					LeaderElectionID:           "controller-runtime",
					LeaderElectionNamespace:    "default",
					LeaderElectionLabels:       labels,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(m).ToNot(BeNil())
				cm, ok := m.(*controllerManager)
				Expect(ok).To(BeTrue())
				ll, isLeaseLock := cm.resourceLock.(*resourcelock.LeaseLock)
				Expect(isLeaseLock).To(BeTrue())
				val, exists := ll.Labels["my-key"]
				Expect(exists).To(BeTrue())
				Expect(val).To(Equal("my-val"))
			})
			When("using a custom LeaderElectionResourceLockInterface", func() {
				It("should use the custom LeaderElectionResourceLockInterface", func() {
					rl, err := fakeleaderelection.NewResourceLock(nil, nil, leaderelection.Options{})
					Expect(err).NotTo(HaveOccurred())

					m, err := New(cfg, Options{
						LeaderElection:                      true,
						LeaderElectionResourceLockInterface: rl,
						newResourceLock: func(config *rest.Config, recorderProvider recorder.Provider, options leaderelection.Options) (resourcelock.Interface, error) {
							return nil, fmt.Errorf("this should not be called")
						},
					})
					Expect(m).ToNot(BeNil())
					Expect(err).ToNot(HaveOccurred())
					cm, ok := m.(*controllerManager)
					Expect(ok).To(BeTrue())
					Expect(cm.resourceLock).To(Equal(rl))
				})
			})
		})

		It("should create a metrics server if a valid address is provided", func(specCtx SpecContext) {
			var srv metricsserver.Server
			m, err := New(cfg, Options{
				Metrics: metricsserver.Options{BindAddress: ":0"},
				newMetricsServer: func(options metricsserver.Options, config *rest.Config, httpClient *http.Client) (metricsserver.Server, error) {
					var err error
					srv, err = metricsserver.NewServer(options, config, httpClient)
					return srv, err
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
			Expect(srv).ToNot(BeNil())

			// Triggering the metric server start here manually to test if it works.
			// Usually this happens later during manager.Start().
			ctx, cancel := context.WithTimeout(specCtx, 5*time.Second)
			Expect(srv.Start(ctx)).To(Succeed())
			cancel()
		})

		It("should create a metrics server if a valid address is provided and secure serving is enabled", func(specCtx SpecContext) {
			var srv metricsserver.Server
			m, err := New(cfg, Options{
				Metrics: metricsserver.Options{BindAddress: ":0", SecureServing: true},
				newMetricsServer: func(options metricsserver.Options, config *rest.Config, httpClient *http.Client) (metricsserver.Server, error) {
					var err error
					srv, err = metricsserver.NewServer(options, config, httpClient)
					return srv, err
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
			Expect(srv).ToNot(BeNil())

			// Triggering the metric server start here manually to test if it works.
			// Usually this happens later during manager.Start().
			ctx, cancel := context.WithTimeout(specCtx, 5*time.Second)
			Expect(srv.Start(ctx)).To(Succeed())
			cancel()
		})

		It("should be able to create a manager with a cache that fails on missing informer", func() {
			m, err := New(cfg, Options{
				Cache: cache.Options{
					ReaderFailOnMissingInformer: true,
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if the metrics bind address is already in use", func(ctx SpecContext) {
			ln, err := net.Listen("tcp", ":0")
			Expect(err).ShouldNot(HaveOccurred())

			var srv metricsserver.Server
			m, err := New(cfg, Options{
				Metrics: metricsserver.Options{
					BindAddress: ln.Addr().String(),
				},
				newMetricsServer: func(options metricsserver.Options, config *rest.Config, httpClient *http.Client) (metricsserver.Server, error) {
					var err error
					srv, err = metricsserver.NewServer(options, config, httpClient)
					return srv, err
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())

			// Triggering the metric server start here manually to test if it works.
			// Usually this happens later during manager.Start().
			Expect(srv.Start(ctx)).ToNot(Succeed())

			Expect(ln.Close()).To(Succeed())
		})

		It("should return an error if the metrics bind address is already in use and secure serving enabled", func(ctx SpecContext) {
			ln, err := net.Listen("tcp", ":0")
			Expect(err).ShouldNot(HaveOccurred())

			var srv metricsserver.Server
			m, err := New(cfg, Options{
				Metrics: metricsserver.Options{
					BindAddress:   ln.Addr().String(),
					SecureServing: true,
				},
				newMetricsServer: func(options metricsserver.Options, config *rest.Config, httpClient *http.Client) (metricsserver.Server, error) {
					var err error
					srv, err = metricsserver.NewServer(options, config, httpClient)
					return srv, err
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())

			// Triggering the metric server start here manually to test if it works.
			// Usually this happens later during manager.Start().
			Expect(srv.Start(ctx)).ToNot(Succeed())

			Expect(ln.Close()).To(Succeed())
		})

		It("should create a listener for the health probes if a valid address is provided", func() {
			var listener net.Listener
			m, err := New(cfg, Options{
				HealthProbeBindAddress: ":0",
				newHealthProbeListener: func(addr string) (net.Listener, error) {
					var err error
					listener, err = defaultHealthProbeListener(addr)
					return listener, err
				},
			})
			Expect(m).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
			Expect(listener).ToNot(BeNil())
			Expect(listener.Close()).ToNot(HaveOccurred())
		})

		It("should return an error if the health probes bind address is already in use", func() {
			ln, err := defaultHealthProbeListener(":0")
			Expect(err).ShouldNot(HaveOccurred())

			var listener net.Listener
			m, err := New(cfg, Options{
				HealthProbeBindAddress: ln.Addr().String(),
				newHealthProbeListener: func(addr string) (net.Listener, error) {
					var err error
					listener, err = defaultHealthProbeListener(addr)
					return listener, err
				},
			})
			Expect(m).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(listener).To(BeNil())

			Expect(ln.Close()).ToNot(HaveOccurred())
		})
	})

	Describe("Start", func() {
		var startSuite = func(options Options, callbacks ...func(Manager)) {
			It("should Start each Component", func(ctx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				var wgRunnableStarted sync.WaitGroup
				wgRunnableStarted.Add(2)
				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					wgRunnableStarted.Done()
					return nil
				}))).To(Succeed())

				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					wgRunnableStarted.Done()
					return nil
				}))).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m.Elected()).ShouldNot(BeClosed())
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()

				<-m.Elected()
				wgRunnableStarted.Wait()
			})

			It("should not manipulate the provided config", func() {
				// strip WrapTransport, cause func values are PartialEq, not Eq --
				// specifically, for reflect.DeepEqual, for all functions F,
				// F != nil implies F != F, which means no full equivalence relation.
				cfg := rest.CopyConfig(cfg)
				cfg.WrapTransport = nil
				originalCfg := rest.CopyConfig(cfg)
				// The options object is shared by multiple tests, copy it
				// into our scope so we manipulate it for this testcase only
				options := options
				options.newResourceLock = nil
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				Expect(m.GetConfig()).To(Equal(originalCfg))
			})

			It("should stop when context is cancelled", func(specCtx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				ctx, cancel := context.WithCancel(specCtx)
				cancel()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			})

			It("should return an error if it can't start the cache", func(ctx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				mgr, ok := m.(*controllerManager)
				Expect(ok).To(BeTrue())
				Expect(mgr.Add(
					&cacheProvider{cache: &informertest.FakeInformers{Error: fmt.Errorf("expected error")}},
				)).To(Succeed())

				Expect(m.Start(ctx)).To(MatchError(ContainSubstring("expected error")))
			})

			It("should start the cache before starting anything else", func(ctx SpecContext) {
				fakeCache := &startSignalingInformer{Cache: &informertest.FakeInformers{}}
				options.NewCache = func(_ *rest.Config, _ cache.Options) (cache.Cache, error) {
					return fakeCache, nil
				}
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				runnableWasStarted := make(chan struct{})
				runnable := RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					if !fakeCache.wasSynced {
						return errors.New("runnable got started before cache was synced")
					}
					close(runnableWasStarted)
					return nil
				})
				Expect(m.Add(runnable)).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).ToNot(HaveOccurred())
				}()

				<-runnableWasStarted
			})

			It("should start additional clusters before anything else", func(ctx SpecContext) {
				fakeCache := &startSignalingInformer{Cache: &informertest.FakeInformers{}}
				options.NewCache = func(_ *rest.Config, _ cache.Options) (cache.Cache, error) {
					return fakeCache, nil
				}
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				additionalClusterCache := &startSignalingInformer{Cache: &informertest.FakeInformers{}}
				additionalCluster, err := cluster.New(cfg, func(o *cluster.Options) {
					o.NewCache = func(_ *rest.Config, _ cache.Options) (cache.Cache, error) {
						return additionalClusterCache, nil
					}
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(m.Add(additionalCluster)).NotTo(HaveOccurred())

				runnableWasStarted := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					if !fakeCache.wasSynced {
						return errors.New("WaitForCacheSyncCalled wasn't called before Runnable got started")
					}
					if !additionalClusterCache.wasSynced {
						return errors.New("the additional clusters WaitForCacheSync wasn't called before Runnable got started")
					}
					close(runnableWasStarted)
					return nil
				}))).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).ToNot(HaveOccurred())
				}()

				<-runnableWasStarted
			})

			It("should return an error if any Components fail to Start", func(ctx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					<-ctx.Done()
					return nil
				}))).To(Succeed())

				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					return fmt.Errorf("expected error")
				}))).To(Succeed())

				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					return nil
				}))).To(Succeed())

				err = m.Start(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("expected error"))
			})

			It("should start caches added after Manager has started", func(ctx SpecContext) {
				fakeCache := &startSignalingInformer{Cache: &informertest.FakeInformers{}}
				options.NewCache = func(_ *rest.Config, _ cache.Options) (cache.Cache, error) {
					return fakeCache, nil
				}
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				runnableWasStarted := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					if !fakeCache.wasSynced {
						return errors.New("WaitForCacheSyncCalled wasn't called before Runnable got started")
					}
					close(runnableWasStarted)
					return nil
				}))).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).ToNot(HaveOccurred())
				}()

				<-runnableWasStarted

				additionalClusterCache := &startSignalingInformer{Cache: &informertest.FakeInformers{}}
				fakeCluster := &startClusterAfterManager{informer: additionalClusterCache}

				Expect(err).NotTo(HaveOccurred())
				Expect(m.Add(fakeCluster)).NotTo(HaveOccurred())

				Eventually(func() bool {
					fakeCluster.informer.mu.Lock()
					defer fakeCluster.informer.mu.Unlock()
					return fakeCluster.informer.wasStarted && fakeCluster.informer.wasSynced
				}).Should(BeTrue())
			})

			It("should wait for runnables to stop", func(specCtx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				var lock sync.Mutex
				var runnableDoneCount int64
				runnableDoneFunc := func() {
					lock.Lock()
					defer lock.Unlock()
					atomic.AddInt64(&runnableDoneCount, 1)
				}
				var wgRunnableRunning sync.WaitGroup
				wgRunnableRunning.Add(2)
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					wgRunnableRunning.Done()
					defer GinkgoRecover()
					defer runnableDoneFunc()
					<-ctx.Done()
					return nil
				}))).To(Succeed())

				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					wgRunnableRunning.Done()
					defer GinkgoRecover()
					defer runnableDoneFunc()
					<-ctx.Done()
					time.Sleep(300 * time.Millisecond) // slow closure simulation
					return nil
				}))).To(Succeed())

				defer GinkgoRecover()
				ctx, cancel := context.WithCancel(specCtx)

				var wgManagerRunning sync.WaitGroup
				wgManagerRunning.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wgManagerRunning.Done()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
					Eventually(func() int64 {
						return atomic.LoadInt64(&runnableDoneCount)
					}).Should(BeEquivalentTo(2))
				}()
				wgRunnableRunning.Wait()
				cancel()

				wgManagerRunning.Wait()
			})

			It("should return an error if any Components fail to Start and wait for runnables to stop", func(ctx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				defer GinkgoRecover()
				var lock sync.Mutex
				runnableDoneCount := 0
				runnableDoneFunc := func() {
					lock.Lock()
					defer lock.Unlock()
					runnableDoneCount++
				}

				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					defer runnableDoneFunc()
					return fmt.Errorf("expected error")
				}))).To(Succeed())

				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					defer GinkgoRecover()
					defer runnableDoneFunc()
					<-ctx.Done()
					return nil
				}))).To(Succeed())

				Expect(m.Start(ctx)).To(HaveOccurred())
				Expect(runnableDoneCount).To(Equal(2))
			})

			It("should refuse to add runnable if stop procedure is already engaged", func(specCtx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				defer GinkgoRecover()

				var wgRunnableRunning sync.WaitGroup
				wgRunnableRunning.Add(1)
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					wgRunnableRunning.Done()
					defer GinkgoRecover()
					<-ctx.Done()
					return nil
				}))).To(Succeed())

				ctx, cancel := context.WithCancel(specCtx)
				go func() {
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				wgRunnableRunning.Wait()
				cancel()
				time.Sleep(100 * time.Millisecond) // give some time for the stop chan closure to be caught by the manager
				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					return nil
				}))).NotTo(Succeed())
			})

			It("should not return runnables context.Canceled errors", func(specCtx SpecContext) {
				Expect(options.Logger).To(BeZero(), "this test overrides Logger")

				var log struct {
					sync.Mutex
					messages []string
				}
				options.Logger = funcr.NewJSON(func(object string) {
					log.Lock()
					log.messages = append(log.messages, object)
					log.Unlock()
				}, funcr.Options{})

				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				// Runnables may return ctx.Err() as shown in some [context.Context] examples.
				started := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					close(started)
					<-ctx.Done()
					return ctx.Err()
				}))).To(Succeed())

				stopped := make(chan error)
				ctx, cancel := context.WithCancel(specCtx)
				go func() {
					stopped <- m.Start(ctx)
				}()

				// Wait for runnables to start, signal the manager, and wait for it to return.
				<-started
				cancel()
				Expect(<-stopped).To(Succeed())

				// The leader election goroutine emits one more log message after Start() returns.
				// Take the lock here to avoid a race between it writing to log.messages and the
				// following read from log.messages.
				if options.LeaderElection {
					log.Lock()
					defer log.Unlock()
				}

				Expect(log.messages).To(Not(ContainElement(
					ContainSubstring(context.Canceled.Error()),
				)))
			})

			It("should default controller logger from manager logger", func(specCtx SpecContext) {
				var lock sync.Mutex
				var messages []string
				options.Logger = funcr.NewJSON(func(object string) {
					lock.Lock()
					messages = append(messages, object)
					lock.Unlock()
				}, funcr.Options{})
				options.LeaderElection = false

				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}

				started := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					close(started)
					return nil
				}))).To(Succeed())

				stopped := make(chan error)
				ctx, cancel := context.WithCancel(specCtx)
				go func() {
					stopped <- m.Start(ctx)
				}()

				// Wait for runnables to start as a proxy for the manager being fully started.
				<-started
				cancel()
				Expect(<-stopped).To(Succeed())

				msg := "controller log message"
				m.GetControllerOptions().Logger.Info(msg)

				Eventually(func(g Gomega) {
					lock.Lock()
					defer lock.Unlock()

					g.Expect(messages).To(ContainElement(
						ContainSubstring(msg),
					))
				}).Should(Succeed())
			})

			It("should return both runnables and stop errors when both error", func(ctx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				m.(*controllerManager).gracefulShutdownTimeout = 1 * time.Nanosecond
				Expect(m.Add(RunnableFunc(func(context.Context) error {
					return runnableError{}
				}))).To(Succeed())
				testDone := make(chan struct{})
				defer close(testDone)
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					timer := time.NewTimer(30 * time.Second)
					defer timer.Stop()
					select {
					case <-testDone:
						return nil
					case <-timer.C:
						return nil
					}
				}))).To(Succeed())
				err = m.Start(ctx)
				Expect(err).To(HaveOccurred())
				eMsg := "[not feeling like that, failed waiting for all runnables to end within grace period of 1ns: context deadline exceeded]"
				Expect(err.Error()).To(Equal(eMsg))
				Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
				Expect(errors.Is(err, runnableError{})).To(BeTrue())
			})

			It("should return only stop errors if runnables dont error", func(specCtx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				m.(*controllerManager).gracefulShutdownTimeout = 1 * time.Nanosecond
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					return nil
				}))).To(Succeed())
				testDone := make(chan struct{})
				defer close(testDone)
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					timer := time.NewTimer(30 * time.Second)
					defer timer.Stop()
					select {
					case <-testDone:
						return nil
					case <-timer.C:
						return nil
					}
				}))).To(Succeed())
				ctx, cancel := context.WithCancel(specCtx)
				managerStopDone := make(chan struct{})
				go func() { err = m.Start(ctx); close(managerStopDone) }()
				// Use the 'elected' channel to find out if startup was done, otherwise we stop
				// before we started the Runnable and see flakes, mostly in low-CPU envs like CI
				<-m.(*controllerManager).elected
				cancel()
				<-managerStopDone
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("failed waiting for all runnables to end within grace period of 1ns: context deadline exceeded"))
				Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
				Expect(errors.Is(err, runnableError{})).ToNot(BeTrue())
			})

			It("should return only runnables error if stop doesn't error", func(ctx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				Expect(m.Add(RunnableFunc(func(context.Context) error {
					return runnableError{}
				}))).To(Succeed())
				err = m.Start(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("not feeling like that"))
				Expect(errors.Is(err, context.DeadlineExceeded)).ToNot(BeTrue())
				Expect(errors.Is(err, runnableError{})).To(BeTrue())
			})

			It("should not wait for runnables if gracefulShutdownTimeout is 0", func(specCtx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				m.(*controllerManager).gracefulShutdownTimeout = time.Duration(0)

				runnableStopped := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					time.Sleep(100 * time.Millisecond)
					close(runnableStopped)
					return nil
				}))).ToNot(HaveOccurred())

				ctx, cancel := context.WithCancel(specCtx)
				managerStopDone := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
					close(managerStopDone)
				}()
				<-m.Elected()
				cancel()

				<-managerStopDone
				<-runnableStopped
			})

			It("should wait forever for runnables if gracefulShutdownTimeout is <0 (-1)", func(specCtx SpecContext) {
				m, err := New(cfg, options)
				Expect(err).NotTo(HaveOccurred())
				for _, cb := range callbacks {
					cb(m)
				}
				m.(*controllerManager).gracefulShutdownTimeout = time.Duration(-1)

				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					time.Sleep(100 * time.Millisecond)
					return nil
				}))).ToNot(HaveOccurred())
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					time.Sleep(200 * time.Millisecond)
					return nil
				}))).ToNot(HaveOccurred())
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					time.Sleep(500 * time.Millisecond)
					return nil
				}))).ToNot(HaveOccurred())
				Expect(m.Add(RunnableFunc(func(ctx context.Context) error {
					<-ctx.Done()
					time.Sleep(1500 * time.Millisecond)
					return nil
				}))).ToNot(HaveOccurred())

				ctx, cancel := context.WithCancel(specCtx)
				managerStopDone := make(chan struct{})
				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
					close(managerStopDone)
				}()
				<-m.Elected()
				cancel()

				beforeDone := time.Now()
				<-managerStopDone
				Expect(time.Since(beforeDone)).To(BeNumerically(">=", 1500*time.Millisecond))
			})

		}

		Context("with defaults", func() {
			startSuite(Options{})
		})

		Context("with leaderelection enabled", func() {
			startSuite(
				Options{
					LeaderElection:          true,
					LeaderElectionID:        "controller-runtime",
					LeaderElectionNamespace: "default",
					newResourceLock:         fakeleaderelection.NewResourceLock,
				},
				func(m Manager) {
					cm, ok := m.(*controllerManager)
					Expect(ok).To(BeTrue())
					cm.onStoppedLeading = func() {}
				},
			)

			It("should return an error if leader election param incorrect", func(specCtx SpecContext) {
				renewDeadline := time.Second * 20
				m, err := New(cfg, Options{
					LeaderElection:          true,
					LeaderElectionID:        "controller-runtime",
					LeaderElectionNamespace: "default",
					newResourceLock:         fakeleaderelection.NewResourceLock,
					RenewDeadline:           &renewDeadline,
				})
				Expect(err).NotTo(HaveOccurred())
				ctx, cancel := context.WithTimeout(specCtx, time.Second*10)
				defer cancel()
				err = m.Start(ctx)
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, context.DeadlineExceeded)).NotTo(BeTrue())
			})
		})

		Context("should start serving metrics", func() {
			var srv metricsserver.Server
			var defaultServer metricsDefaultServer
			var opts Options

			BeforeEach(func() {
				srv = nil
				opts = Options{
					Metrics: metricsserver.Options{
						BindAddress: ":0",
					},
					newMetricsServer: func(options metricsserver.Options, config *rest.Config, httpClient *http.Client) (metricsserver.Server, error) {
						var err error
						srv, err = metricsserver.NewServer(options, config, httpClient)
						if srv != nil {
							defaultServer = srv.(metricsDefaultServer)
						}
						return srv, err
					},
				}
			})

			It("should stop serving metrics when stop is called", func(specCtx SpecContext) {
				m, err := New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				ctx, cancel := context.WithCancel(specCtx)
				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				// Check the metrics started
				endpoint := fmt.Sprintf("http://%s/metrics", defaultServer.GetBindAddr())
				_, err = http.Get(endpoint)
				Expect(err).NotTo(HaveOccurred())

				// Shutdown the server
				cancel()

				// Expect the metrics server to shutdown
				Eventually(func() error {
					_, err = http.Get(endpoint)
					return err
				}, 10*time.Second).ShouldNot(Succeed())
			})

			It("should serve metrics endpoint", func(ctx SpecContext) {
				m, err := New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				metricsEndpoint := fmt.Sprintf("http://%s/metrics", defaultServer.GetBindAddr())
				resp, err := http.Get(metricsEndpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))
			})

			It("should not serve anything other than metrics endpoint by default", func(ctx SpecContext) {
				m, err := New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				endpoint := fmt.Sprintf("http://%s/should-not-exist", defaultServer.GetBindAddr())
				resp, err := http.Get(endpoint)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(404))
			})

			It("should serve metrics in its registry", func(ctx SpecContext) {
				one := prometheus.NewCounter(prometheus.CounterOpts{
					Name: "test_one",
					Help: "test metric for testing",
				})
				one.Inc()
				err := metrics.Registry.Register(one)
				Expect(err).NotTo(HaveOccurred())

				m, err := New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				metricsEndpoint := fmt.Sprintf("http://%s/metrics", defaultServer.GetBindAddr())
				resp, err := http.Get(metricsEndpoint)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
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
				m, err := New(cfg, opts)
				Expect(err).NotTo(HaveOccurred())

				// Should error when we add another extra endpoint on the already registered path.
				err = m.AddMetricsServerExtraHandler("/debug", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					_, _ = w.Write([]byte("Another debug info"))
				}))
				Expect(err).To(HaveOccurred())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()
				// Note: Wait until metrics server has been started. A finished leader election
				// doesn't guarantee that the metrics server is up.
				Eventually(func() string { return defaultServer.GetBindAddr() }, 10*time.Second).ShouldNot(BeEmpty())

				endpoint := fmt.Sprintf("http://%s/debug", defaultServer.GetBindAddr())
				resp, err := http.Get(endpoint)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal("Some debug info"))
			})
		})
	})

	Context("should start serving health probes", func() {
		var listener net.Listener
		var opts Options

		BeforeEach(func() {
			listener = nil
			opts = Options{
				newHealthProbeListener: func(addr string) (net.Listener, error) {
					var err error
					listener, err = defaultHealthProbeListener(addr)
					return listener, err
				},
			}
		})

		AfterEach(func() {
			if listener != nil {
				listener.Close()
			}
		})

		It("should stop serving health probes when stop is called", func(specCtx SpecContext) {
			opts.HealthProbeBindAddress = ":0"
			m, err := New(cfg, opts)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(specCtx)
			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()
			<-m.Elected()

			// Check the health probes started
			endpoint := fmt.Sprintf("http://%s", listener.Addr().String())
			_, err = http.Get(endpoint)
			Expect(err).NotTo(HaveOccurred())

			// Shutdown the server
			cancel()

			// Expect the health probes server to shutdown
			Eventually(func() error {
				_, err = http.Get(endpoint)
				return err
			}, 10*time.Second).ShouldNot(Succeed())
		})

		It("should serve readiness endpoint", func(ctx SpecContext) {
			opts.HealthProbeBindAddress = ":0"
			m, err := New(cfg, opts)
			Expect(err).NotTo(HaveOccurred())

			res := fmt.Errorf("not ready yet")
			namedCheck := "check"
			err = m.AddReadyzCheck(namedCheck, func(_ *http.Request) error { return res })
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()
			<-m.Elected()

			readinessEndpoint := fmt.Sprint("http://", listener.Addr().String(), defaultReadinessEndpoint)

			// Controller is not ready
			resp, err := http.Get(readinessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			// Controller is ready
			res = nil
			resp, err = http.Get(readinessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Check readiness path without trailing slash without redirect
			readinessEndpoint = fmt.Sprint("http://", listener.Addr().String(), defaultReadinessEndpoint)
			res = nil
			httpClient := http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse // Do not follow redirect
				},
			}
			resp, err = httpClient.Get(readinessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Check readiness path for individual check
			readinessEndpoint = fmt.Sprint("http://", listener.Addr().String(), path.Join(defaultReadinessEndpoint, namedCheck))
			res = nil
			resp, err = http.Get(readinessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should serve liveness endpoint", func(ctx SpecContext) {
			opts.HealthProbeBindAddress = ":0"
			m, err := New(cfg, opts)
			Expect(err).NotTo(HaveOccurred())

			res := fmt.Errorf("not alive")
			namedCheck := "check"
			err = m.AddHealthzCheck(namedCheck, func(_ *http.Request) error { return res })
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()
			<-m.Elected()

			livenessEndpoint := fmt.Sprint("http://", listener.Addr().String(), defaultLivenessEndpoint)

			// Controller is not ready
			resp, err := http.Get(livenessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			// Controller is ready
			res = nil
			resp, err = http.Get(livenessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Check liveness path without trailing slash without redirect
			livenessEndpoint = fmt.Sprint("http://", listener.Addr().String(), defaultLivenessEndpoint)
			res = nil
			httpClient := http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse // Do not follow redirect
				},
			}
			resp, err = httpClient.Get(livenessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Check readiness path for individual check
			livenessEndpoint = fmt.Sprint("http://", listener.Addr().String(), path.Join(defaultLivenessEndpoint, namedCheck))
			res = nil
			resp, err = http.Get(livenessEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Context("should start serving pprof", func() {
		var listener net.Listener
		var opts Options

		BeforeEach(func() {
			listener = nil
			opts = Options{
				newPprofListener: func(addr string) (net.Listener, error) {
					var err error
					listener, err = defaultPprofListener(addr)
					return listener, err
				},
			}
		})

		AfterEach(func() {
			if listener != nil {
				listener.Close()
			}
		})

		It("should stop serving pprof when stop is called", func(specCtx SpecContext) {
			opts.PprofBindAddress = ":0"
			m, err := New(cfg, opts)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithCancel(specCtx)
			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()
			<-m.Elected()

			// Check the pprof started
			endpoint := fmt.Sprintf("http://%s", listener.Addr().String())
			_, err = http.Get(endpoint)
			Expect(err).NotTo(HaveOccurred())

			// Shutdown the server
			cancel()

			// Expect the pprof server to shutdown
			Eventually(func() error {
				_, err = http.Get(endpoint)
				return err
			}, 10*time.Second).ShouldNot(Succeed())
		})

		It("should serve pprof endpoints", func(ctx SpecContext) {
			opts.PprofBindAddress = ":0"
			m, err := New(cfg, opts)
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()
			<-m.Elected()

			pprofIndexEndpoint := fmt.Sprintf("http://%s/debug/pprof/", listener.Addr().String())
			resp, err := http.Get(pprofIndexEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			pprofCmdlineEndpoint := fmt.Sprintf("http://%s/debug/pprof/cmdline", listener.Addr().String())
			resp, err = http.Get(pprofCmdlineEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			pprofProfileEndpoint := fmt.Sprintf("http://%s/debug/pprof/profile", listener.Addr().String())
			resp, err = http.Get(pprofProfileEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			pprofSymbolEndpoint := fmt.Sprintf("http://%s/debug/pprof/symbol", listener.Addr().String())
			resp, err = http.Get(pprofSymbolEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			pprofTraceEndpoint := fmt.Sprintf("http://%s/debug/pprof/trace", listener.Addr().String())
			resp, err = http.Get(pprofTraceEndpoint)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Add", func() {
		It("should immediately start the Component if the Manager has already Started another Component",
			func(ctx SpecContext) {
				m, err := New(cfg, Options{})
				Expect(err).NotTo(HaveOccurred())
				mgr, ok := m.(*controllerManager)
				Expect(ok).To(BeTrue())

				// Add one component before starting
				c1 := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					close(c1)
					return nil
				}))).To(Succeed())

				go func() {
					defer GinkgoRecover()
					Expect(m.Start(ctx)).NotTo(HaveOccurred())
				}()
				<-m.Elected()

				// Wait for the Manager to start
				Eventually(func() bool {
					return mgr.runnables.Caches.Started()
				}).Should(BeTrue())

				// Add another component after starting
				c2 := make(chan struct{})
				Expect(m.Add(RunnableFunc(func(context.Context) error {
					defer GinkgoRecover()
					close(c2)
					return nil
				}))).To(Succeed())
				<-c1
				<-c2
			})

		It("should immediately start the Component if the Manager has already Started", func(ctx SpecContext) {
			m, err := New(cfg, Options{})
			Expect(err).NotTo(HaveOccurred())
			mgr, ok := m.(*controllerManager)
			Expect(ok).To(BeTrue())

			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()

			// Wait for the Manager to start
			Eventually(func() bool {
				return mgr.runnables.Caches.Started()
			}).Should(BeTrue())

			c1 := make(chan struct{})
			Expect(m.Add(RunnableFunc(func(context.Context) error {
				defer GinkgoRecover()
				close(c1)
				return nil
			}))).To(Succeed())
			<-c1
		})

		It("should fail if attempted to start a second time", func(ctx SpecContext) {
			m, err := New(cfg, Options{})
			Expect(err).NotTo(HaveOccurred())

			go func() {
				defer GinkgoRecover()
				Expect(m.Start(ctx)).NotTo(HaveOccurred())
			}()
			// Wait for the Manager to start
			Eventually(func() bool {
				mgr, ok := m.(*controllerManager)
				Expect(ok).To(BeTrue())
				return mgr.runnables.Caches.Started()
			}).Should(BeTrue())

			err = m.Start(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("manager already started"))

		})
	})

	It("should not leak goroutines when stopped", func(specCtx SpecContext) {
		currentGRs := goleak.IgnoreCurrent()

		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(specCtx)
		cancel()
		Expect(m.Start(ctx)).NotTo(HaveOccurred())

		// force-close keep-alive connections.  These'll time anyway (after
		// like 30s or so) but force it to speed up the tests.
		clientTransport.CloseIdleConnections()
		Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
	})

	It("should not leak goroutines if the default event broadcaster is used & events are emitted", func(specCtx SpecContext) {
		currentGRs := goleak.IgnoreCurrent()

		m, err := New(cfg, Options{ /* implicit: default setting for EventBroadcaster */ })
		Expect(err).NotTo(HaveOccurred())

		By("adding a runnable that emits an event")
		ns := corev1.Namespace{}
		ns.Name = "default"

		recorder := m.GetEventRecorderFor("rock-and-roll")
		Expect(m.Add(RunnableFunc(func(_ context.Context) error {
			recorder.Event(&ns, "Warning", "BallroomBlitz", "yeah, yeah, yeah-yeah-yeah")
			return nil
		}))).To(Succeed())

		By("starting the manager & waiting till we've sent our event")
		ctx, cancel := context.WithCancel(specCtx)
		doneCh := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(doneCh)
			Expect(m.Start(ctx)).To(Succeed())
		}()
		<-m.Elected()

		Eventually(func() *corev1.Event {
			evts, err := clientset.CoreV1().Events("").SearchWithContext(ctx, m.GetScheme(), &ns)
			Expect(err).NotTo(HaveOccurred())

			for i, evt := range evts.Items {
				if evt.Reason == "BallroomBlitz" {
					return &evts.Items[i]
				}
			}
			return nil
		}).ShouldNot(BeNil())

		By("making sure there's no extra go routines still running after we stop")
		cancel()
		<-doneCh

		// force-close keep-alive connections.  These'll time anyway (after
		// like 30s or so) but force it to speed up the tests.
		clientTransport.CloseIdleConnections()
		Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
	})

	It("should not leak goroutines when a runnable returns error slowly after being signaled to stop", func(specCtx SpecContext) {
		// This test reproduces the race condition where the manager's Start method
		// exits due to context cancellation, leaving no one to drain errChan

		currentGRs := goleak.IgnoreCurrent()

		// Create manager with a very short graceful shutdown timeout to reliablytrigger the race condition
		shortGracefulShutdownTimeout := 10 * time.Millisecond
		m, err := New(cfg, Options{
			GracefulShutdownTimeout: &shortGracefulShutdownTimeout,
		})
		Expect(err).NotTo(HaveOccurred())

		// Add the slow runnable that will return an error after some delay
		for i := 0; i < 3; i++ {
			slowRunnable := RunnableFunc(func(c context.Context) error {
				<-c.Done()

				// Simulate some work that delays the error from being returned
				// Choosing a large delay to reliably trigger the race condition
				time.Sleep(100 * time.Millisecond)

				// This simulates the race condition where runnables try to send
				// errors after the manager has stopped reading from errChan
				return errors.New("slow runnable error")
			})

			Expect(m.Add(slowRunnable)).To(Succeed())
		}

		ctx, cancel := context.WithTimeout(specCtx, 50*time.Millisecond)
		defer cancel()
		go func() {
			defer GinkgoRecover()
			Expect(m.Start(ctx)).To(HaveOccurred()) // We expect error here because the slow runnables will return errors
		}()

		// Wait for context to be cancelled
		<-ctx.Done()

		// Give time for any leaks to become apparent. This makes sure that we don't false alarm on go routine leaks because runnables are still running.
		time.Sleep(300 * time.Millisecond)

		// force-close keep-alive connections
		clientTransport.CloseIdleConnections()
		Eventually(func() error { return goleak.Find(currentGRs) }).Should(Succeed())
	})

	It("should provide a function to get the Config", func() {
		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())
		mgr, ok := m.(*controllerManager)
		Expect(ok).To(BeTrue())
		Expect(m.GetConfig()).To(Equal(mgr.cluster.GetConfig()))
	})

	It("should provide a function to get the Client", func() {
		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())
		mgr, ok := m.(*controllerManager)
		Expect(ok).To(BeTrue())
		Expect(m.GetClient()).To(Equal(mgr.cluster.GetClient()))
	})

	It("should provide a function to get the Scheme", func() {
		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())
		mgr, ok := m.(*controllerManager)
		Expect(ok).To(BeTrue())
		Expect(m.GetScheme()).To(Equal(mgr.cluster.GetScheme()))
	})

	It("should provide a function to get the FieldIndexer", func() {
		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())
		mgr, ok := m.(*controllerManager)
		Expect(ok).To(BeTrue())
		Expect(m.GetFieldIndexer()).To(Equal(mgr.cluster.GetFieldIndexer()))
	})

	It("should provide a function to get the EventRecorder", func() {
		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())
		Expect(m.GetEventRecorderFor("test")).NotTo(BeNil())
	})
	It("should provide a function to get the APIReader", func() {
		m, err := New(cfg, Options{})
		Expect(err).NotTo(HaveOccurred())
		Expect(m.GetAPIReader()).NotTo(BeNil())
	})

	It("should run warmup runnables before leader election is won", func(ctx SpecContext) {
		By("Creating a channel to track execution order")
		runnableExecutionOrderChan := make(chan string, 2)
		const leaderElectionRunnableName = "leaderElectionRunnable"
		const warmupRunnableName = "warmupRunnable"

		By("Creating a manager with leader election enabled")
		m, err := New(cfg, Options{
			LeaderElection:          true,
			LeaderElectionNamespace: "default",
			LeaderElectionID:        "test-leader-election-warmup",
			newResourceLock:         fakeleaderelection.NewResourceLock,
			HealthProbeBindAddress:  "0",
			Metrics:                 metricsserver.Options{BindAddress: "0"},
			PprofBindAddress:        "0",
		})
		Expect(err).NotTo(HaveOccurred())

		By("Creating a runnable that implements WarmupRunnable interface")
		// Create a warmup runnable
		warmupRunnable := newWarmupRunnableFunc(
			func(ctx context.Context) error {
				// This is the leader election runnable that will be executed after leader election
				// It will block until context is done/cancelled
				<-ctx.Done()
				return nil
			},
			func(ctx context.Context) error {
				// This should be called during startup before leader election
				runnableExecutionOrderChan <- warmupRunnableName
				return nil
			},
		)
		Expect(m.Add(warmupRunnable)).To(Succeed())

		By("Creating a runnable that requires leader election")
		leaderElectionRunnable := RunnableFunc(
			func(ctx context.Context) error {
				runnableExecutionOrderChan <- leaderElectionRunnableName
				<-ctx.Done()
				return nil
			},
		)
		Expect(m.Add(leaderElectionRunnable)).To(Succeed())

		cm, ok := m.(*controllerManager)
		Expect(ok).To(BeTrue())
		resourceLockWithHooks, ok := cm.resourceLock.(fakeleaderelection.ControllableResourceLockInterface)
		Expect(ok).To(BeTrue())

		By("Blocking leader election")
		resourceLockWithHooks.BlockLeaderElection()

		By("Starting the manager")
		go func() {
			defer GinkgoRecover()
			Expect(m.Start(ctx)).To(Succeed())
		}()

		By("Waiting for the warmup runnable to be executed without leader election being won")
		Expect(<-runnableExecutionOrderChan).To(Equal(warmupRunnableName))

		By("Unblocking leader election")
		resourceLockWithHooks.UnblockLeaderElection()

		By("Waiting for the leader election runnable to be executed after leader election was won")
		<-m.Elected()
		Expect(<-runnableExecutionOrderChan).To(Equal(leaderElectionRunnableName))
	})
})

type runnableError struct {
}

func (runnableError) Error() string {
	return "not feeling like that"
}

var _ Runnable = &cacheProvider{}

type cacheProvider struct {
	cache cache.Cache
}

func (c *cacheProvider) GetCache() cache.Cache {
	return c.cache
}

func (c *cacheProvider) Start(ctx context.Context) error {
	return c.cache.Start(ctx)
}

type startSignalingInformer struct {
	mu sync.Mutex

	// The manager calls Start and WaitForCacheSync in
	// parallel, so we have to protect wasStarted with a Mutex
	// and block in WaitForCacheSync until it is true.
	wasStarted bool
	// was synced will be true once Start was called and
	// WaitForCacheSync returned, just like a real cache.
	wasSynced bool
	cache.Cache
}

func (c *startSignalingInformer) Start(ctx context.Context) error {
	c.mu.Lock()
	c.wasStarted = true
	c.mu.Unlock()
	return c.Cache.Start(ctx)
}

func (c *startSignalingInformer) WaitForCacheSync(ctx context.Context) bool {
	defer func() {
		c.mu.Lock()
		c.wasSynced = true
		c.mu.Unlock()
	}()
	return c.Cache.WaitForCacheSync(ctx)
}

type startClusterAfterManager struct {
	informer *startSignalingInformer
}

func (c *startClusterAfterManager) Start(ctx context.Context) error {
	return c.informer.Start(ctx)
}

func (c *startClusterAfterManager) GetCache() cache.Cache {
	return c.informer
}

// metricsDefaultServer is used to type check the default metrics server implementation
// so we can retrieve the bind addr without having to make GetBindAddr a function on the
// metricsserver.Server interface or resort to reflection.
type metricsDefaultServer interface {
	GetBindAddr() string
}

type needElection struct {
	ch chan struct{}
}

func (n *needElection) Start(_ context.Context) error {
	n.ch <- struct{}{}
	return nil
}

func (n *needElection) NeedLeaderElection() bool {
	return true
}
