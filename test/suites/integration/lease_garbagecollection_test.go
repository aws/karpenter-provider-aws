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

package integration_test

import (
	"time"

	coordinationsv1 "k8s.io/api/coordination/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Lease Garbage Collection", func() {
	var badLease *coordinationsv1.Lease
	BeforeEach(func() {
		badLease = &coordinationsv1.Lease{
			ObjectMeta: v1.ObjectMeta{
				CreationTimestamp: v1.Time{Time: time.Now().Add(-time.Hour * 2)},
				Name:              "new-lease",
				Namespace:         "kube-node-lease",
				Labels:            map[string]string{test.DiscoveryLabel: "unspecified"},
			},
		}
	})
	It("should delete node lease that does not contain an OwnerReference", func() {
		env.ExpectCreated(badLease)
		env.EventuallyExpectNotFound(badLease)
	})
})
