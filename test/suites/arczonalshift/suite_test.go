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

package arczonalshift_test

import (
	"fmt"
	"math/rand"
	"testing"

	arczonalshiftservice "github.com/aws/aws-sdk-go-v2/service/arczonalshift"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
)

var env *environmentaws.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestZonalShift(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "ZonalShift")
}

var _ = BeforeEach(func() {
	env.Context = options.ToContext(env.Context, test.Options(test.OptionsFields{
		EnableZonalShift: lo.ToPtr(true),
	}))
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Zonal Shift", func() {
	var clusterArn string
	var zoneid string
	var zonalshiftid *string
	var subnetInfo []environmentaws.SubnetInfo
	BeforeEach(func() {
		clusterArn = fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", env.Region, env.ExpectAccountID(), env.ClusterName)
		subnetInfo = lo.UniqBy(env.GetSubnetInfo(map[string]string{"karpenter.sh/discovery": env.ClusterName}), func(s environmentaws.SubnetInfo) string {
			return s.Zone
		})
		zoneid = subnetInfo[rand.Intn(len(subnetInfo))].ZoneID //nolint:gosec
	})
	It("should update cache when a zonal shift is detected", func() {
		By("using the Zonal Shift Provider to check zonal shift status on Zonal Shift start")

		startzonalshiftresponse, starterr := env.ARCZONALSHIFTAPI.StartZonalShift(env.Context, &arczonalshiftservice.StartZonalShiftInput{
			ResourceIdentifier: lo.ToPtr(clusterArn),
			AwayFrom:           lo.ToPtr(zoneid),
			ExpiresIn:          lo.ToPtr("1h"),
			Comment:            lo.ToPtr("karpenter e2e test"),
		})
		zonalshiftid = startzonalshiftresponse.ZonalShiftId
		Expect(starterr).ToNot(HaveOccurred())
		env.EventuallyExpectClusterToZonalShift(zoneid)
		DeferCleanup(func() {
			_, err := env.ARCZONALSHIFTAPI.CancelZonalShift(env.Context, &arczonalshiftservice.CancelZonalShiftInput{
				ZonalShiftId: zonalshiftid,
			})
			Expect(err).ToNot(HaveOccurred())
			env.EventuallyExpectClusterToNotHaveZonalShift(zoneid)
		})
		By("using the Zonal Shift Provider to check zonal shift status on Zonal Shift end")

		_, cancelerr := env.ARCZONALSHIFTAPI.CancelZonalShift(env.Context, &arczonalshiftservice.CancelZonalShiftInput{
			ZonalShiftId: zonalshiftid,
		})
		Expect(cancelerr).ToNot(HaveOccurred())
		env.EventuallyExpectClusterToNotHaveZonalShift(zoneid)
	})
})
