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

package storage_test

import (
	"testing"

	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/storage"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		storage.Env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		storage.Env.Stop()
	})
	RunSpecs(t, "Storage")
}

var _ = BeforeEach(func() {
	storage.Env.BeforeEach()
	storage.NodeClass = storage.Env.DefaultEC2NodeClass()
	storage.NodePool = storage.Env.DefaultNodePool(storage.NodeClass)
})
var _ = AfterEach(func() { storage.Env.Cleanup() })
var _ = AfterEach(func() { storage.Env.AfterEach() })

var _ = storage.RegisterTests()
