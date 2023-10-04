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

package nodeclass_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/record"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/events"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/controllers/nodeclass"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment
var opts options.Options
var nodeTemplateController corecontroller.Controller
var nodeClassController corecontroller.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSNodeTemplateController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...), coretest.WithFieldIndexers(test.EC2NodeClassFieldIndexer(ctx)))
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	awsEnv = test.NewEnvironment(ctx, env)

	nodeTemplateController = nodeclass.NewNodeTemplateController(env.Client, events.NewRecorder(&record.FakeRecorder{}), awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider)
	nodeClassController = nodeclass.NewNodeClassController(env.Client, events.NewRecorder(&record.FakeRecorder{}), awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})
