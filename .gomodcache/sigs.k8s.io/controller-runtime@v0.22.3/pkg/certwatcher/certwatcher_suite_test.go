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

package certwatcher_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	certPath = "testdata/tls.crt"
	keyPath  = "testdata/tls.key"
)

func TestSource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CertWatcher Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})

var _ = AfterSuite(func() {
	for _, file := range []string{certPath, keyPath, certPath + ".new", keyPath + ".new", certPath + ".old", keyPath + ".old"} {
		_ = os.Remove(file)
	}
})
