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

package client_test

import (
	"bytes"
	"io"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/examples/crd/pkg"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var (
	testenv   *envtest.Environment
	cfg       *rest.Config
	clientset *kubernetes.Clientset

	// Used by tests to inspect controller and client log messages.
	log bytes.Buffer
)

var _ = BeforeSuite(func() {
	// Forwards logs to ginkgo output, and allows tests to inspect logs.
	mw := io.MultiWriter(&log, GinkgoWriter)

	// Use prefixes to help us tell the source of the log message.
	// controller-runtime uses logf
	logf.SetLogger(zap.New(zap.WriteTo(mw), zap.UseDevMode(true)).WithName("logf"))
	// client-go logs uses klog
	klog.SetLogger(zap.New(zap.WriteTo(mw), zap.UseDevMode(true)).WithName("klog"))

	testenv = &envtest.Environment{CRDDirectoryPaths: []string{"./testdata"}}

	var err error
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	Expect(pkg.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testenv.Stop()).To(Succeed())
})
