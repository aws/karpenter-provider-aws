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

package controlplane_test

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ccapi "k8s.io/client-go/tools/clientcmd/api"

	. "sigs.k8s.io/controller-runtime/pkg/internal/testing/controlplane"
)

var _ = Describe("Kubectl", func() {
	It("runs kubectl", func() {
		k := &KubeCtl{Path: "bash"}
		args := []string{"-c", "echo 'something'"}
		stdout, stderr, err := k.Run(args...)
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(ContainSubstring("something"))
		bytes, err := io.ReadAll(stderr)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes).To(BeEmpty())
	})

	Context("when the command returns a non-zero exit code", func() {
		It("returns an error", func() {
			k := &KubeCtl{Path: "bash"}
			args := []string{
				"-c", "echo 'this is StdErr' >&2; echo 'but this is StdOut' >&1; exit 66",
			}

			stdout, stderr, err := k.Run(args...)

			Expect(err).To(MatchError(ContainSubstring("exit status 66")))

			Expect(stdout).To(ContainSubstring("but this is StdOut"))
			Expect(stderr).To(ContainSubstring("this is StdErr"))
		})
	})
})

var _ = Describe("KubeConfigFromREST", func() {
	var (
		restCfg *rest.Config
		rawCfg  []byte
		cfg     *ccapi.Config
	)

	BeforeEach(func() {
		restCfg = &rest.Config{
			Host:    "https://some-host:8675",
			APIPath: "/some-prefix",
			TLSClientConfig: rest.TLSClientConfig{
				CertData: []byte("cert"),
				KeyData:  []byte("key"),
				CAData:   []byte("ca-cert"),
			},
			BearerToken: "some-tok",
			Username:    "some-user",
			Password:    "some-password",
		}
	})

	JustBeforeEach(func() {
		var err error
		rawCfg, err = KubeConfigFromREST(restCfg)
		Expect(err).NotTo(HaveOccurred(), "should be able to convert & serialize the kubeconfig")

		cfg, err = clientcmd.Load(rawCfg)
		Expect(err).NotTo(HaveOccurred(), "should be able to deserialize the generated kubeconfig")
	})

	It("should set up a context, and set it as the current one", func() {
		By("checking that the current context exists")
		Expect(cfg.CurrentContext).NotTo(BeEmpty(), "should have a current context")
		Expect(cfg.Contexts).To(HaveKeyWithValue(cfg.CurrentContext, Not(BeNil())), "the current context should exist as a context")

		By("checking that it points to valid info")
		currCtx := cfg.Contexts[cfg.CurrentContext]
		Expect(currCtx).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Cluster":  Not(BeEmpty()),
			"AuthInfo": Not(BeEmpty()),
		})))

		Expect(cfg.Clusters).To(HaveKeyWithValue(currCtx.Cluster, Not(BeNil())), "should point to a cluster")
		Expect(cfg.AuthInfos).To(HaveKeyWithValue(currCtx.AuthInfo, Not(BeNil())), "should point to a user")
	})

	Context("when no TLS is enabled", func() {
		BeforeEach(func() {
			restCfg.Host = "http://some-host:8675"
			restCfg.TLSClientConfig = rest.TLSClientConfig{}
		})

		It("should use http in the server url", func() {
			cluster := cfg.Clusters[cfg.Contexts[cfg.CurrentContext].Cluster]
			Expect(cluster.Server).To(HavePrefix("http://"))
		})
	})

	It("configure the current context to point to the given REST config's server, with CA data", func() {
		cluster := cfg.Clusters[cfg.Contexts[cfg.CurrentContext].Cluster]
		Expect(cluster).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Server":                   Equal("https://some-host:8675/some-prefix"),
			"CertificateAuthorityData": Equal([]byte("ca-cert")),
		})))
	})

	It("should copy all non-plugin auth info over", func() {
		user := cfg.AuthInfos[cfg.Contexts[cfg.CurrentContext].AuthInfo]
		Expect(user).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"ClientCertificateData": Equal([]byte("cert")),
			"ClientKeyData":         Equal([]byte("key")),
			"Token":                 Equal("some-tok"),
			"Username":              Equal("some-user"),
			"Password":              Equal("some-password"),
		})))
	})
})
