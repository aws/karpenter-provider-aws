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

package recorder_test

import (
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestRecorder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Recorder Integration Suite")
}

var testenv *envtest.Environment
var cfg *rest.Config
var httpClient *http.Client
var clientset *kubernetes.Clientset

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testenv = &envtest.Environment{}

	var err error
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())

	httpClient, err = rest.HTTPClientFor(cfg)
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testenv.Stop()).To(Succeed())
})
