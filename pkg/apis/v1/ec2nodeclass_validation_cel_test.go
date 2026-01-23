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

package v1_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/aws"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CEL/Validation", func() {
	var nc *v1.EC2NodeClass

	BeforeEach(func() {
		nc = &v1.EC2NodeClass{
			ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
			Spec: v1.EC2NodeClassSpec{
				AMIFamily:        &v1.AMIFamilyAL2023,
				AMISelectorTerms: []v1.AMISelectorTerm{{Alias: "al2023@latest"}},
				Role:             "role-1",
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
			},
		}
	})
	It("should succeed if just specifying role", func() {
		Expect(env.Client.Create(ctx, nc)).To(Succeed())
	})
	It("should succeed if just specifying instance profile", func() {
		nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		nc.Spec.Role = ""
		Expect(env.Client.Create(ctx, nc)).To(Succeed())
	})
	It("should fail if specifying both instance profile and role", func() {
		nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
	})
	It("should fail if not specifying one of instance profile and role", func() {
		nc.Spec.Role = ""
		Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
	})
	Context("UserData", func() {
		It("should succeed if user data is empty", func() {
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
	})
	Context("AMIFamily", func() {
		amiFamilies := []string{v1.AMIFamilyAL2, v1.AMIFamilyAL2023, v1.AMIFamilyBottlerocket, v1.AMIFamilyWindows2019, v1.AMIFamilyWindows2022, v1.AMIFamilyWindows2025, v1.AMIFamilyCustom}
		DescribeTable("should succeed with valid families", func() []interface{} {
			f := func(amiFamily string) {
				// Set a custom AMI family so it's compatible with all ami family types
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-0123456789abcdef"}}
				nc.Spec.AMIFamily = lo.ToPtr(amiFamily)
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			}
			entries := lo.Map(amiFamilies, func(family string, _ int) any {
				return Entry(family, family)
			})
			return append([]any{f}, entries...)
		}()...)
		It("should fail with the ubuntu family", func() {
			// Set a custom AMI family so it's compatible with all ami family types
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-0123456789abcdef"}}
			nc.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyUbuntu)
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		DescribeTable("should succeed when the amiFamily matches amiSelectorTerms[].alias", func() []any {
			f := func(amiFamily, alias string) {
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				nc.Spec.AMIFamily = lo.ToPtr(amiFamily)
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			}
			entries := lo.FilterMap(amiFamilies, func(family string, _ int) (any, bool) {
				if family == v1.AMIFamilyCustom {
					return nil, false
				}
				alias := fmt.Sprintf("%s@latest", strings.ToLower(family))
				return Entry(
					fmt.Sprintf("family %q with alias %q", family, alias),
					family,
					alias,
				), true
			})
			return append([]any{f}, entries...)
		}()...)
		DescribeTable("should succeed when the amiFamily is custom with amiSelectorTerms[].alias", func() []any {
			f := func(alias string) {
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				nc.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			}
			entries := lo.FilterMap(amiFamilies, func(family string, _ int) (any, bool) {
				if family == v1.AMIFamilyCustom {
					return nil, false
				}
				alias := fmt.Sprintf("%s@latest", strings.ToLower(family))
				return Entry(
					fmt.Sprintf(`family "Custom" with alias %q`, alias),
					alias,
				), true
			})
			return append([]any{f}, entries...)
		}()...)
		DescribeTable("should fail when then amiFamily does not match amiSelectorTerms[].alias", func() []any {
			f := func(amiFamily, alias string) {
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				nc.Spec.AMIFamily = lo.ToPtr(amiFamily)
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			}
			entries := []any{}
			families := lo.Reject(amiFamilies, func(family string, _ int) bool {
				return family == v1.AMIFamilyCustom
			})
			for i := range families {
				for j := range families {
					if i == j {
						continue
					}
					alias := fmt.Sprintf("%s@latest", strings.ToLower(families[j]))
					entries = append(entries, Entry(
						fmt.Sprintf("family %q with alias %q", families[i], alias),
						families[i],
						alias,
					))
				}

			}
			return append([]any{f}, entries...)
		}()...)
		It("should fail when neither amiFamily nor an alias are specified", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-01234567890abcdef"}}
			nc.Spec.AMIFamily = nil
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
	})
	Context("Tags", func() {
		It("should succeed when tags are empty", func() {
			nc.Spec.Tags = map[string]string{}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed if tags aren't in restricted tag keys", func() {
			nc.Spec.Tags = map[string]string{
				"karpenter.sh/custom-key": "value",
				"karpenter.sh/managed":    "true",
				"kubernetes.io/role/key":  "value",
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail if tags contain a restricted domain key", func() {
			nc.Spec.Tags = map[string]string{
				karpv1.NodePoolLabelKey: "value",
			}
			Expect(env.Client.Create(ctx, nc)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				"kubernetes.io/cluster/test": "value",
			}
			Expect(env.Client.Create(ctx, nc)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				v1.EKSClusterNameTagKey: "test",
			}
			Expect(env.Client.Create(ctx, nc)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				v1.LabelNodeClass: "test",
			}
			Expect(env.Client.Create(ctx, nc)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				"karpenter.sh/nodeclaim": "test",
			}
			Expect(env.Client.Create(ctx, nc)).To(Not(Succeed()))
		})
	})
	Context("SubnetSelectorTerms", func() {
		It("should succeed with a valid subnet selector on tags", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid subnet selector on id", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID: "subnet-12345749",
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail when subnet selector terms is set to nil", func() {
			nc.Spec.SubnetSelectorTerms = nil
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when no subnet selector terms exist", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has no values", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has no tag map values", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has a tag map key that is empty", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"test": "",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has a tag map value that is empty", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when the last subnet selector is invalid", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with tags", func() {
			nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID: "subnet-12345749",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
	})
	Context("SecurityGroupSelectorTerms", func() {
		It("should succeed with a valid security group selector on tags", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid security group selector on id", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					ID: "sg-12345749",
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid security group selector on name", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Name: "testname",
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail when security group selector terms is set to nil", func() {
			nc.Spec.SecurityGroupSelectorTerms = nil
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when no security group selector terms exist", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has no values", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has no tag map values", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has a tag map key that is empty", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"test": "",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has a tag map value that is empty", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when the last security group selector is invalid", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with tags", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					ID: "sg-12345749",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with name", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					ID:   "sg-12345749",
					Name: "my-security-group",
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying name with tags", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Name: "my-security-group",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
	})
	Context("CapacityReservationSelectorTerms", func() {
		It("should succeed with a valid capacity reservation selector on tags", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				Tags: map[string]string{
					"test": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid capacity reservation selector on id", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				ID: "cr-12345749",
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed for a valid ownerID", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				OwnerID: "012345678901",
				Tags: map[string]string{
					"test": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail with a capacity reservation selector on a malformed id", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				ID: "r-12345749",
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should succeed when capacity group selector terms is set to nil", func() {
			nc.Spec.CapacityReservationSelectorTerms = nil
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail when a capacity reservation selector term has no values", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a capacity reservation selector term has no tag map values", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				Tags: map[string]string{},
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a capacity reservation selector term has a tag map key that is empty", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				Tags: map[string]string{
					"test": "",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a capacity reservation selector term has a tag map value that is empty", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				Tags: map[string]string{
					"": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when the last capacity reservation selector is invalid", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with tags in a single term", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				ID: "cr-12345749",
				Tags: map[string]string{
					"test": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with ownerID in a single term", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				OwnerID: "012345678901",
				ID:      "cr-12345749",
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when the ownerID is malformed", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				OwnerID: "01234567890", // OwnerID must be 12 digits, this is 11
				Tags: map[string]string{
					"test": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when the ownerID is set by itself", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				OwnerID: "012345678901",
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should succeed with a valid instanceMatchCriteria 'open'", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				InstanceMatchCriteria: "open",
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid instanceMatchCriteria 'targeted'", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				InstanceMatchCriteria: "targeted",
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with instanceMatchCriteria combined with tags", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				InstanceMatchCriteria: "open",
				Tags: map[string]string{
					"test": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with instanceMatchCriteria combined with ownerID and tags", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				InstanceMatchCriteria: "targeted",
				OwnerID:               "012345678901",
				Tags: map[string]string{
					"test": "testvalue",
				},
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail with invalid instanceMatchCriteria value", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				InstanceMatchCriteria: "invalid",
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with instanceMatchCriteria", func() {
			nc.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
				ID:                    "cr-12345749",
				InstanceMatchCriteria: "open",
			}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
	})
	Context("AMISelectorTerms", func() {
		It("should succeed with a valid ami selector on alias", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
				Alias: "al2023@latest",
			}}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid ami selector on tags", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid ami selector on id", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID: "ami-12345749",
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid ami selector on name", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Name: "testname",
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed with a valid ami selector on name and owner", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Name:  "testname",
					Owner: "testowner",
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should succeed when an ami selector term has an owner key with tags", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Owner: "testowner",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail when a ami selector term has no values", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a ami selector term has no tag map values", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a ami selector term has a tag map key that is empty", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"test": "",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when a ami selector term has a tag map value that is empty", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when the last ami selector is invalid", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		DescribeTable(
			"should fail when specifying id with other fields",
			func(mutation v1.AMISelectorTerm) {
				term := v1.AMISelectorTerm{ID: "ami-1234749"}
				Expect(mergo.Merge(&term, &mutation)).To(Succeed())
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{term}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			},
			Entry("alias", v1.AMISelectorTerm{Alias: "al2023@latest"}),
			Entry("tags", v1.AMISelectorTerm{
				Tags: map[string]string{"test": "testvalue"},
			}),
			Entry("name", v1.AMISelectorTerm{Name: "my-custom-ami"}),
			Entry("owner", v1.AMISelectorTerm{Owner: "123456789"}),
		)
		DescribeTable(
			"should fail when specifying alias with other fields",
			func(mutation v1.AMISelectorTerm) {
				term := v1.AMISelectorTerm{Alias: "al2023@latest"}
				Expect(mergo.Merge(&term, &mutation)).To(Succeed())
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{term}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			},
			Entry("id", v1.AMISelectorTerm{ID: "ami-1234749"}),
			Entry("tags", v1.AMISelectorTerm{
				Tags: map[string]string{"test": "testvalue"},
			}),
			Entry("name", v1.AMISelectorTerm{Name: "my-custom-ami"}),
			Entry("owner", v1.AMISelectorTerm{Owner: "123456789"}),
		)
		It("should fail when specifying alias with other terms", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{Alias: "al2023@latest"},
				{ID: "ami-1234749"},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		DescribeTable(
			"should succeed for valid aliases",
			func(alias, family string) {
				nc.Spec.AMIFamily = lo.ToPtr(family)
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			},
			Entry("al2 (latest)", "al2@latest", v1.AMIFamilyAL2),
			Entry("al2 (pinned)", "al2@v20240625", v1.AMIFamilyAL2),
			Entry("al2023 (latest)", "al2023@latest", v1.AMIFamilyAL2023),
			Entry("al2023 (pinned)", "al2023@v20240625", v1.AMIFamilyAL2023),
			Entry("bottlerocket (latest)", "bottlerocket@latest", v1.AMIFamilyBottlerocket),
			Entry("bottlerocket (pinned)", "bottlerocket@1.10.0", v1.AMIFamilyBottlerocket),
			Entry("windows2019 (latest)", "windows2019@latest", v1.AMIFamilyWindows2019),
			Entry("windows2022 (latest)", "windows2022@latest", v1.AMIFamilyWindows2022),
			Entry("windows2025 (latest)", "windows2025@latest", v1.AMIFamilyWindows2025),
		)
		DescribeTable(
			"should fail for incorrectly formatted aliases",
			func(aliases ...string) {
				for _, alias := range aliases {
					nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
					Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
				}
			},
			Entry("missing family", "@latest"),
			Entry("missing version", "al2023", "al2023@"),
			Entry("invalid separator", "al2023-latest"),
		)
		It("should fail for an alias with an invalid family", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "ubuntu@latest"}}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		DescribeTable(
			"should fail when specifying non-latest versions with Windows aliases",
			func(alias string) {
				nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			},
			Entry("Windows2019", "windows2019@v1.0.0"),
			Entry("Windows2022", "windows2022@v1.0.0"),
			Entry("Windows2025", "windows2025@v1.0.0"),
		)
	})
	Context("Kubelet", func() {
		It("should fail on kubeReserved with invalid keys", func() {
			nc.Spec.Kubelet = &v1.KubeletConfiguration{
				KubeReserved: map[string]string{
					string(corev1.ResourcePods): "2",
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail on systemReserved with invalid keys", func() {
			nc.Spec.Kubelet = &v1.KubeletConfiguration{
				SystemReserved: map[string]string{
					string(corev1.ResourcePods): "2",
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		Context("Eviction Signals", func() {
			Context("Eviction Hard", func() {
				It("should succeed on evictionHard with valid keys", func() {
					nc.Spec.Kubelet = &v1.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available":   "5%",
							"nodefs.available":   "10%",
							"nodefs.inodesFree":  "15%",
							"imagefs.available":  "5%",
							"imagefs.inodesFree": "5%",
							"pid.available":      "5%",
						},
					}
					Expect(env.Client.Create(ctx, nc)).To(Succeed())
				})
				It("should fail on evictionHard with invalid keys", func() {
					nc.Spec.Kubelet = &v1.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory": "5%",
						},
					}
					Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
				})
				It("should fail on invalid formatted percentage value in evictionHard", func() {
					nc.Spec.Kubelet = &v1.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available": "5%3",
						},
					}
					Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
				})
				It("should fail on invalid percentage value (too large) in evictionHard", func() {
					nc.Spec.Kubelet = &v1.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available": "110%",
						},
					}
					Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
				})
				It("should fail on invalid quantity value in evictionHard", func() {
					nc.Spec.Kubelet = &v1.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available": "110GB",
						},
					}
					Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
				})
			})
		})
		Context("Eviction Soft", func() {
			It("should succeed on evictionSoft with valid keys", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available":   "5%",
						"nodefs.available":   "10%",
						"nodefs.inodesFree":  "15%",
						"imagefs.available":  "5%",
						"imagefs.inodesFree": "5%",
						"pid.available":      "5%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available":   {Duration: time.Minute},
						"nodefs.available":   {Duration: time.Second * 90},
						"nodefs.inodesFree":  {Duration: time.Minute * 5},
						"imagefs.available":  {Duration: time.Hour},
						"imagefs.inodesFree": {Duration: time.Hour * 24},
						"pid.available":      {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			})
			It("should fail on evictionSoft with invalid keys", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory": "5%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory": {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail on invalid formatted percentage value in evictionSoft", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "5%3",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail on invalid percentage value (too large) in evictionSoft", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "110%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail on invalid quantity value in evictionSoft", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "110GB",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail when eviction soft doesn't have matching grace period", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "200Mi",
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
		})
		Context("GCThresholdPercent", func() {
			It("should succeed on a valid imageGCHighThresholdPercent", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCHighThresholdPercent: lo.ToPtr(int32(10)),
				}
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			})
			It("should fail when imageGCHighThresholdPercent is less than imageGCLowThresholdPercent", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCHighThresholdPercent: lo.ToPtr(int32(50)),
					ImageGCLowThresholdPercent:  lo.ToPtr(int32(60)),
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail when imageGCLowThresholdPercent is greather than imageGCHighThresheldPercent", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCHighThresholdPercent: lo.ToPtr(int32(50)),
					ImageGCLowThresholdPercent:  lo.ToPtr(int32(60)),
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
		})
		Context("Eviction Soft Grace Period", func() {
			It("should succeed on evictionSoftGracePeriod with valid keys", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available":   "5%",
						"nodefs.available":   "10%",
						"nodefs.inodesFree":  "15%",
						"imagefs.available":  "5%",
						"imagefs.inodesFree": "5%",
						"pid.available":      "5%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available":   {Duration: time.Minute},
						"nodefs.available":   {Duration: time.Second * 90},
						"nodefs.inodesFree":  {Duration: time.Minute * 5},
						"imagefs.available":  {Duration: time.Hour},
						"imagefs.inodesFree": {Duration: time.Hour * 24},
						"pid.available":      {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			})
			It("should fail on evictionSoftGracePeriod with invalid keys", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory": {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail when eviction soft grace period doesn't have matching threshold", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
		})
	})
	Context("MetadataOptions", func() {
		It("should succeed for valid inputs", func() {
			nc.Spec.MetadataOptions = &v1.MetadataOptions{
				HTTPEndpoint:            aws.String("disabled"),
				HTTPProtocolIPv6:        aws.String("enabled"),
				HTTPPutResponseHopLimit: aws.Int64(34),
				HTTPTokens:              aws.String("optional"),
			}
			Expect(env.Client.Create(ctx, nc)).To(Succeed())
		})
		It("should fail for invalid for HTTPEndpoint", func() {
			nc.Spec.MetadataOptions = &v1.MetadataOptions{
				HTTPEndpoint: aws.String("test"),
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail for invalid for HTTPProtocolIPv6", func() {
			nc.Spec.MetadataOptions = &v1.MetadataOptions{
				HTTPProtocolIPv6: aws.String("test"),
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail for invalid for HTTPPutResponseHopLimit", func() {
			nc.Spec.MetadataOptions = &v1.MetadataOptions{
				HTTPPutResponseHopLimit: aws.Int64(-5),
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail for invalid for HTTPTokens", func() {
			nc.Spec.MetadataOptions = &v1.MetadataOptions{
				HTTPTokens: aws.String("test"),
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
	})
	Context("BlockDeviceMappings", func() {
		It("should succeed if more than one root volume is specified", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(500, resource.Giga),
							},

							RootVolume: true,
						},
						{
							DeviceName: aws.String("map-device-2"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(50, resource.Tera),
							},

							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Succeed())
		})
		It("should succeed for valid VolumeSize in G", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(58, resource.Giga),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Succeed())
		})
		It("should succeed for valid VolumeSize in T", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(45, resource.Tera),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Succeed())
		})
		It("should fail if more than one root volume is specified", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
							},
							RootVolume: true,
						},
						{
							DeviceName: aws.String("map-device-2"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
							},
							RootVolume: true,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
		It("should fail VolumeSize is less then 1Gi/1G", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(1, resource.Milli),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
		It("should fail VolumeSize is greater then 64T", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(100, resource.Tera),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
		It("should fail for VolumeSize that do not parse into quantity values", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize: &resource.Quantity{},
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
		It("should fail if VolumeInitializationRate set but SnapshotID not specified", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize:               &resource.Quantity{},
								VolumeInitializationRate: aws.Int32(100),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
		It("should fail if VolumeInitializationRate too low", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize:               &resource.Quantity{},
								SnapshotID:               aws.String("snap-1"),
								VolumeInitializationRate: aws.Int32(99),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
		It("should fail if VolumeInitializationRate too high", func() {
			nodeClass := &v1.EC2NodeClass{
				ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
				Spec: v1.EC2NodeClassSpec{
					AMISelectorTerms:           nc.Spec.AMISelectorTerms,
					SubnetSelectorTerms:        nc.Spec.SubnetSelectorTerms,
					SecurityGroupSelectorTerms: nc.Spec.SecurityGroupSelectorTerms,
					Role:                       nc.Spec.Role,
					BlockDeviceMappings: []*v1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1.BlockDevice{
								VolumeSize:               &resource.Quantity{},
								SnapshotID:               aws.String("snap-1"),
								VolumeInitializationRate: aws.Int32(888),
							},
							RootVolume: false,
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, nodeClass)).To(Not(Succeed()))
		})
	})
	Context("Role Immutability", func() {
		It("should fail if role is not defined", func() {
			nc.Spec.Role = ""
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should succeed when updating the role", func() {
			nc.Spec.Role = "test-role"
			Expect(env.Client.Create(ctx, nc)).To(Succeed())

			nc.Spec.Role = "test-role2"
			Expect(env.Client.Update(ctx, nc)).To(Succeed())
		})
		It("should succeed to switch between an unmanaged and managed instance profile", func() {
			nc.Spec.Role = ""
			nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Create(ctx, nc)).To(Succeed())

			nc.Spec.Role = "test-role"
			nc.Spec.InstanceProfile = nil
			Expect(env.Client.Update(ctx, nc)).To(Succeed())
		})
		It("should succeed to switch between a managed and unmanaged instance profile", func() {
			nc.Spec.Role = "test-role"
			nc.Spec.InstanceProfile = nil
			Expect(env.Client.Create(ctx, nc)).To(Succeed())

			nc.Spec.Role = ""
			nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Update(ctx, nc)).To(Succeed())
		})
	})
})
