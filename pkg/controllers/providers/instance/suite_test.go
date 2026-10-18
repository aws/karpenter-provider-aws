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

package instance_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	controllersinstance "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *controllersinstance.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceCache")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(coretest.NodeClaimProviderIDFieldIndexer(ctx)))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	controller = controllersinstance.NewController(awsEnv.InstanceProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

// managedInstance returns an EC2 instance carrying the tags SyncCache filters on so the fake DescribeInstances returns it.
func managedInstance() ec2types.Instance {
	return test.EC2Instance(ec2types.Instance{
		Placement: &ec2types.Placement{
			AvailabilityZone:   aws.String("test-zone-1a"),
			AvailabilityZoneId: aws.String("tstz1-1a"),
		},
		Tags: []ec2types.Tag{
			{Key: aws.String(v1.NodePoolTagKey), Value: aws.String("default")},
			{Key: aws.String(v1.LabelNodeClass), Value: aws.String("default")},
			{Key: aws.String(v1.EKSClusterNameTagKey), Value: aws.String(options.FromContext(ctx).ClusterName)},
		},
	})
}

var _ = Describe("Instance Cache Controller", func() {
	It("should populate the instance cache from EC2 on reconcile", func() {
		inst := managedInstance()
		id := aws.ToString(inst.InstanceId)
		awsEnv.EC2API.Instances.Store(id, inst)

		ExpectSingletonReconciled(ctx, controller)

		// After the controller reconciles, the cache is populated and List returns the instance.
		instances, err := awsEnv.InstanceProvider.List(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(lo.Map(instances, func(i *instance.Instance, _ int) string { return i.ID })).To(ConsistOf(id))
	})
	It("should lazily populate the cache on the first List before the controller has run", func() {
		inst := managedInstance()
		id := aws.ToString(inst.InstanceId)
		awsEnv.EC2API.Instances.Store(id, inst)

		// No controller reconcile yet: the first List lazily syncs the cache rather than returning an empty fleet.
		instances, err := awsEnv.InstanceProvider.List(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(lo.Map(instances, func(i *instance.Instance, _ int) string { return i.ID })).To(ConsistOf(id))
	})
	It("should evict cache entries for instances EC2 no longer returns", func() {
		inst := managedInstance()
		id := aws.ToString(inst.InstanceId)
		awsEnv.EC2API.Instances.Store(id, inst)

		ExpectSingletonReconciled(ctx, controller)
		instances, err := awsEnv.InstanceProvider.List(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(instances).To(HaveLen(1))

		// Remove the instance from EC2 and reconcile again; the stale entry should be evicted.
		awsEnv.EC2API.Instances.Delete(id)
		ExpectSingletonReconciled(ctx, controller)

		_, err = awsEnv.InstanceProvider.Get(ctx, id)
		Expect(corecloudprovider.IsNodeClaimNotFoundError(err)).To(BeTrue())
	})
	It("should expose seconds since the last successful sync as a metric", func() {
		ExpectSingletonReconciled(ctx, controller)

		// The gauge is computed at scrape time, so right after a sync it should read a small, non-negative age.
		metric, ok := FindMetricWithLabelValues("karpenter_instance_cache_seconds_since_last_sync", map[string]string{})
		Expect(ok).To(BeTrue())
		Expect(metric.GetGauge().GetValue()).To(BeNumerically(">=", 0))
		Expect(metric.GetGauge().GetValue()).To(BeNumerically("<", 60))
	})
})
