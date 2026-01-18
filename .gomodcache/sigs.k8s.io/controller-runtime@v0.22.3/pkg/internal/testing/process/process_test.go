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

package process_test

import (
	"bytes"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"sigs.k8s.io/controller-runtime/pkg/internal/testing/addr"
	. "sigs.k8s.io/controller-runtime/pkg/internal/testing/process"
)

const (
	healthURLPath = "/healthz"
)

var _ = Describe("Start method", func() {
	var (
		processState *State
		server       *ghttp.Server
	)
	BeforeEach(func() {
		server = ghttp.NewServer()

		processState = &State{
			Path: "bash",
			Args: simpleBashScript,
			HealthCheck: HealthCheck{
				URL: getServerURL(server),
			},
		}
		processState.Path = "bash"
		processState.Args = simpleBashScript

	})
	AfterEach(func() {
		server.Close()
	})

	Context("when process takes too long to start", func() {
		BeforeEach(func() {
			server.RouteToHandler("GET", healthURLPath, func(resp http.ResponseWriter, _ *http.Request) {
				time.Sleep(250 * time.Millisecond)
				resp.WriteHeader(http.StatusOK)
			})
		})
		It("returns a timeout error", func() {
			processState.StartTimeout = 200 * time.Millisecond

			err := processState.Start(nil, nil)
			Expect(err).To(MatchError(ContainSubstring("timeout")))

			Eventually(func() bool { done, _ := processState.Exited(); return done }).Should(BeTrue())
		})
	})

	Context("when the healthcheck returns ok", func() {
		BeforeEach(func() {

			server.RouteToHandler("GET", healthURLPath, ghttp.RespondWith(http.StatusOK, ""))
		})

		It("can start a process", func() {
			processState.StartTimeout = 10 * time.Second

			err := processState.Start(nil, nil)
			Expect(err).NotTo(HaveOccurred())

			Consistently(processState.Exited).Should(BeFalse())
		})

		It("hits the endpoint, and successfully starts", func() {
			processState.StartTimeout = 100 * time.Millisecond

			err := processState.Start(nil, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(server.ReceivedRequests()).To(HaveLen(1))
			Consistently(processState.Exited).Should(BeFalse())
		})

		Context("when the command cannot be started", func() {
			var err error

			BeforeEach(func() {
				processState = &State{}
				processState.Path = "/nonexistent"

				err = processState.Start(nil, nil)
			})

			It("propagates the error", func() {
				Expect(os.IsNotExist(err)).To(BeTrue())
			})

			Context("but Stop() is called on it", func() {
				It("does not panic", func() {
					stoppingFailedProcess := func() {
						Expect(processState.Stop()).To(Succeed())
					}

					Expect(stoppingFailedProcess).NotTo(Panic())
				})
			})
		})

		Context("when IO is configured", func() {
			It("can inspect stdout & stderr", func() {
				stdout := &bytes.Buffer{}
				stderr := &bytes.Buffer{}

				processState.Args = []string{
					"-c",
					`
						echo 'this is stderr' >&2
						echo 'that is stdout'
						echo 'i started' >&2
					`,
				}
				processState.StartTimeout = 5 * time.Second

				Expect(processState.Start(stdout, stderr)).To(Succeed())
				Eventually(processState.Exited).Should(BeTrue())

				Expect(stdout.String()).To(Equal("that is stdout\n"))
				Expect(stderr.String()).To(Equal("this is stderr\ni started\n"))
			})
		})
	})

	Context("when the healthcheck always returns failure", func() {
		BeforeEach(func() {
			server.RouteToHandler("GET", healthURLPath, ghttp.RespondWith(http.StatusInternalServerError, ""))
		})
		It("returns a timeout error and stops health API checker", func() {
			processState.HealthCheck.URL = getServerURL(server)
			processState.HealthCheck.Path = healthURLPath
			processState.StartTimeout = 500 * time.Millisecond

			err := processState.Start(nil, nil)
			Expect(err).To(MatchError(ContainSubstring("timeout")))

			nrReceivedRequests := len(server.ReceivedRequests())
			Expect(nrReceivedRequests).To(Equal(5))
			time.Sleep(200 * time.Millisecond)
			Expect(nrReceivedRequests).To(Equal(5))
		})
	})

	Context("when the healthcheck isn't even listening", func() {
		BeforeEach(func() {
			server.Close()
		})

		It("returns a timeout error", func() {
			processState.HealthCheck.Path = healthURLPath
			processState.StartTimeout = 500 * time.Millisecond

			port, host, err := addr.Suggest("")
			Expect(err).NotTo(HaveOccurred())

			processState.HealthCheck.URL = url.URL{
				Scheme: "http",
				Host:   net.JoinHostPort(host, strconv.Itoa(port)),
			}

			err = processState.Start(nil, nil)
			Expect(err).To(MatchError(ContainSubstring("timeout")))
		})
	})

	Context("when the healthcheck fails initially but succeeds eventually", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.RespondWith(http.StatusInternalServerError, ""),
				ghttp.RespondWith(http.StatusInternalServerError, ""),
				ghttp.RespondWith(http.StatusInternalServerError, ""),
				ghttp.RespondWith(http.StatusOK, ""),
			)
		})

		It("hits the endpoint repeatedly, and successfully starts", func() {
			processState.HealthCheck.URL = getServerURL(server)
			processState.HealthCheck.Path = healthURLPath
			processState.StartTimeout = 20 * time.Second

			err := processState.Start(nil, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(server.ReceivedRequests()).To(HaveLen(4))
			Consistently(processState.Exited).Should(BeFalse())
		})

		Context("when the polling interval is not configured", func() {
			It("uses the default interval for polling", func() {
				processState.HealthCheck.URL = getServerURL(server)
				processState.HealthCheck.Path = "/helathz"
				processState.StartTimeout = 300 * time.Millisecond

				Expect(processState.Start(nil, nil)).To(MatchError(ContainSubstring("timeout")))
				Expect(server.ReceivedRequests()).To(HaveLen(3))
			})
		})

		Context("when the polling interval is configured", func() {
			BeforeEach(func() {
				processState.HealthCheck.URL = getServerURL(server)
				processState.HealthCheck.Path = healthURLPath
				processState.HealthCheck.PollInterval = time.Millisecond * 150
			})

			It("hits the endpoint in the configured interval", func() {
				processState.StartTimeout = 3 * processState.HealthCheck.PollInterval

				Expect(processState.Start(nil, nil)).To(MatchError(ContainSubstring("timeout")))
				Expect(server.ReceivedRequests()).To(HaveLen(3))
			})
		})
	})
})

var _ = Describe("Stop method", func() {
	var (
		server       *ghttp.Server
		processState *State
	)
	BeforeEach(func() {
		server = ghttp.NewServer()
		server.RouteToHandler("GET", healthURLPath, ghttp.RespondWith(http.StatusOK, ""))
		processState = &State{
			Path: "bash",
			Args: simpleBashScript,
			HealthCheck: HealthCheck{
				URL: getServerURL(server),
			},
		}
		processState.StartTimeout = 10 * time.Second
	})

	AfterEach(func() {
		server.Close()
	})
	Context("when Stop() is called", func() {
		BeforeEach(func() {
			Expect(processState.Start(nil, nil)).To(Succeed())
			processState.StopTimeout = 10 * time.Second
		})

		It("stops the process", func() {
			Expect(processState.Stop()).To(Succeed())
		})

		Context("multiple times", func() {
			It("does not error or panic on consecutive calls", func() {
				stoppingTheProcess := func() {
					Expect(processState.Stop()).To(Succeed())
				}
				Expect(stoppingTheProcess).NotTo(Panic())
				Expect(stoppingTheProcess).NotTo(Panic())
				Expect(stoppingTheProcess).NotTo(Panic())
			})
		})
	})

	Context("when the command cannot be stopped", func() {
		It("returns a timeout error", func() {
			Expect(processState.Start(nil, nil)).To(Succeed())
			processState.StopTimeout = 1 * time.Nanosecond // much shorter than the sleep in the script

			Expect(processState.Stop()).To(MatchError(ContainSubstring("timeout")))
		})
	})

	Context("when the directory needs to be cleaned up", func() {
		It("removes the directory", func() {
			var err error

			Expect(processState.Start(nil, nil)).To(Succeed())
			processState.Dir, err = os.MkdirTemp("", "k8s_test_framework_")
			Expect(err).NotTo(HaveOccurred())
			processState.DirNeedsCleaning = true
			processState.StopTimeout = 400 * time.Millisecond

			Expect(processState.Stop()).To(Succeed())
			Expect(processState.Dir).NotTo(BeAnExistingFile())
		})
	})
})

var _ = Describe("Init", func() {
	Context("when all inputs are provided", func() {
		It("passes them through", func() {
			ps := &State{
				Dir:          "/some/dir",
				Path:         "/some/path/to/some/bin",
				StartTimeout: 20 * time.Hour,
				StopTimeout:  65537 * time.Millisecond,
			}

			Expect(ps.Init("some name")).To(Succeed())

			Expect(ps.Dir).To(Equal("/some/dir"))
			Expect(ps.DirNeedsCleaning).To(BeFalse())
			Expect(ps.Path).To(Equal("/some/path/to/some/bin"))
			Expect(ps.StartTimeout).To(Equal(20 * time.Hour))
			Expect(ps.StopTimeout).To(Equal(65537 * time.Millisecond))
		})
	})

	Context("when inputs are empty", func() {
		It("ps them", func() {
			ps := &State{}
			Expect(ps.Init("some name")).To(Succeed())

			Expect(ps.Dir).To(BeADirectory())
			Expect(os.RemoveAll(ps.Dir)).To(Succeed())
			Expect(ps.DirNeedsCleaning).To(BeTrue())

			Expect(ps.Path).NotTo(BeEmpty())

			Expect(ps.StartTimeout).NotTo(BeZero())
			Expect(ps.StopTimeout).NotTo(BeZero())
		})
	})

	Context("when neither name nor path are provided", func() {
		It("returns an error", func() {
			ps := &State{}
			Expect(ps.Init("")).To(MatchError("must have at least one of name or path"))
		})
	})
})

var simpleBashScript = []string{
	"-c", "tail -f /dev/null",
}

func getServerURL(server *ghttp.Server) url.URL {
	url, err := url.Parse(server.URL())
	Expect(err).NotTo(HaveOccurred())
	url.Path = healthURLPath
	return *url
}
