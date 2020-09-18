/*
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

package test

import (
	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/ellistarn/karpenter/pkg/utils/project"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func init() {
	log.Setup()
}

// Environment encapsulates bring up and tear down of a testing environment
func Environment(setupFn func(controllerruntime.Manager) error) (client.Client, chan struct{}) {
	var err error
	stop := make(chan struct{})
	scheme := pkgruntime.NewScheme()
	gomega.Expect(apis.AddToScheme(scheme)).To(gomega.Succeed(), "Failed to initailize apis scheme")
	gomega.Expect(clientgoscheme.AddToScheme(scheme)).To(gomega.Succeed(), "Failed to initailize clientgo scheme")

	environment := &envtest.Environment{
		CRDDirectoryPaths: []string{project.RelativeToRoot("config/crd/bases")},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			DirectoryPaths: []string{project.RelativeToRoot("config/webhook")},
		},
	}

	_, err = environment.Start()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "Failed to start Environment")
	manager, err := controllerruntime.NewManager(environment.Config, controllerruntime.Options{
		CertDir: environment.WebhookInstallOptions.LocalServingCertDir,
		Host:    environment.WebhookInstallOptions.LocalServingHost,
		Port:    environment.WebhookInstallOptions.LocalServingPort,
		Scheme:  scheme,
	})
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "Failed to initialize Manager")
	gomega.Expect(setupFn(manager)).To(gomega.Succeed(), "Failed to execute setupFn")

	go func() {
		defer ginkgo.GinkgoRecover()
		gomega.Expect(manager.Start(stop)).To(gomega.Succeed(), "Failed to stop Manager")
		gomega.Expect(environment.Stop()).To(gomega.Succeed())
	}()

	return manager.GetClient(), stop
}
