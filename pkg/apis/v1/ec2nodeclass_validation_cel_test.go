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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CEL/Validation", func() {
	var nc *v1.EC2NodeClass

	BeforeEach(func() {
		if env.Version.Minor() < 25 {
			Skip("CEL Validation is for 1.25>")
		}
		nc = &v1.EC2NodeClass{
			ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
			Spec: v1.EC2NodeClassSpec{
				AMIFamily: lo.ToPtr(v1.AMIFamilyAL2023),
				Role:      "role-1",
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
				karpv1.ManagedByAnnotationKey: "test",
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
	Context("AMISelectorTerms", func() {
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
		It("should fail when specifying id with tags", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID: "ami-12345749",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with name", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID:   "ami-12345749",
					Name: "my-custom-ami",
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when specifying id with owner", func() {
			nc.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID:    "ami-12345749",
					Owner: "123456789",
				},
			}
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when AMIFamily is Custom and not AMISelectorTerms", func() {
			nc.Spec.AMIFamily = &v1.AMIFamilyCustom
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
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
					ImageGCHighThresholdPercent: ptr.Int32(10),
				}
				Expect(env.Client.Create(ctx, nc)).To(Succeed())
			})
			It("should fail when imageGCHighThresholdPercent is less than imageGCLowThresholdPercent", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCHighThresholdPercent: ptr.Int32(50),
					ImageGCLowThresholdPercent:  ptr.Int32(60),
				}
				Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
			})
			It("should fail when imageGCLowThresholdPercent is greather than imageGCHighThresheldPercent", func() {
				nc.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCHighThresholdPercent: ptr.Int32(50),
					ImageGCLowThresholdPercent:  ptr.Int32(60),
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
					AMIFamily:                  nc.Spec.AMIFamily,
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
	})
	Context("Role Immutability", func() {
		It("should fail if role is not defined", func() {
			nc.Spec.Role = ""
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail when updating the role", func() {
			nc.Spec.Role = "test-role"
			Expect(env.Client.Create(ctx, nc)).To(Succeed())

			nc.Spec.Role = "test-role2"
			Expect(env.Client.Create(ctx, nc)).ToNot(Succeed())
		})
		It("should fail to switch between an unmanaged and managed instance profile", func() {
			nc.Spec.Role = ""
			nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Create(ctx, nc)).To(Succeed())

			nc.Spec.Role = "test-role"
			nc.Spec.InstanceProfile = nil
			Expect(env.Client.Update(ctx, nc)).ToNot(Succeed())
		})
		It("should fail to switch between a managed and unmanaged instance profile", func() {
			nc.Spec.Role = "test-role"
			nc.Spec.InstanceProfile = nil
			Expect(env.Client.Create(ctx, nc)).To(Succeed())

			nc.Spec.Role = ""
			nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Update(ctx, nc)).ToNot(Succeed())
		})
	})
})
