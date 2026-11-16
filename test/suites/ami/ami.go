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

package ami

import (
	"embed"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/status"
	. "github.com/awslabs/operatorpkg/test/expectations" //nolint:staticcheck
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	coretest "sigs.k8s.io/karpenter/pkg/test"
)

//go:embed testdata
var testdata embed.FS

var Env *environmentaws.Environment
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	Describe("AMI", func() {
		var ssmPath string
		var customAMI string
		var deprecatedAMI string

		BeforeEach(func() {
			ssmPath = fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", Env.K8sVersion())
			customAMI = Env.GetAMIBySSMPath(ssmPath)
			deprecatedAMI = Env.GetDeprecatedAMI(customAMI, "AL2023")
		})

		It("should use the AMI defined by the AMI Selector Terms", func() {
			pod := coretest.Pod()
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: customAMI}}
			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should use the most recent AMI when discovering multiple", func() {
			// choose an old static image that will definitely have an older creation date
			oldCustomAMI := Env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%[1]s/amazon-linux-2023/x86_64/standard/amazon-eks-node-al2023-x86_64-standard-%[1]s-v20260923/image_id", Env.K8sVersion()))
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{ID: customAMI},
				{ID: oldCustomAMI},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should support AMI Selector Terms for Name but fail with incorrect owners", func() {
			output, err := Env.EC2API.DescribeImages(Env.Context, &ec2.DescribeImagesInput{
				ImageIds: []string{customAMI},
			})
			Expect(err).To(BeNil())
			Expect(output.Images).To(HaveLen(1))
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Name:  *output.Images[0].Name,
					Owner: "fakeOwnerValue",
				},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.ExpectCreatedNodeCount("==", 0)
			Expect(pod.Spec.NodeName).To(Equal(""))
		})
		It("should support ami selector Name with default owners", func() {
			output, err := Env.EC2API.DescribeImages(Env.Context, &ec2.DescribeImagesInput{
				ImageIds: []string{customAMI},
			})
			Expect(err).To(BeNil())
			Expect(output.Images).To(HaveLen(1))

			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					Name: *output.Images[0].Name,
				},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should support ami selector ids", func() {
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID: customAMI,
				},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should support ssm parameters by ARN", func() {
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					SSMParameter: fmt.Sprintf("arn:aws:ssm:%s::parameter%s", Env.Region, ssmPath),
				},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})
		It("should support launching nodes with a deprecated ami", func() {
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID: deprecatedAMI,
				},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(deprecatedAMI))))

			nc := EventuallyExpectAMIsToExist(NodeClass)
			Expect(len(nc.Status.AMIs)).To(BeNumerically("==", 1))
			Expect(nc.Status.AMIs[0].Deprecated).To(BeTrue())
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeAMIsReady, Status: metav1.ConditionTrue})
		})
		It("should prioritize launch with non-deprecated AMIs", func() {
			NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
				{
					ID: deprecatedAMI,
				},
				{
					ID: customAMI,
				},
			}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(customAMI))))
		})

		Context("AMIFamily", func() {
			DescribeTable(
				"should provision a node using the latest alias",
				func(alias string) {
					if strings.HasPrefix(alias, "al2@") && Env.K8sMinorVersion() > 32 {
						Skip("AL2 is not supported on versions > 1.32")
					}
					pod := coretest.Pod()
					NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
					Env.ExpectCreated(NodeClass, NodePool, pod)
					Env.EventuallyExpectHealthy(pod)
					Env.ExpectCreatedNodeCount("==", 1)
				},
				Entry("AL2023 (latest)", "al2023@latest"),
				Entry("AL2 (latest)", "al2@latest"),
				Entry("Bottlerocket (latest)", "bottlerocket@latest"),
			)
			DescribeTable(
				"should provision a node using a pinned alias",
				func(amiFamily string, withV bool) {
					if amiFamily == "al2" && Env.K8sMinorVersion() > 32 {
						Skip("AL2 is not supported on versions > 1.32")
					}
					amiVersion := Env.GetPinnedAMIVersion(amiFamily)
					if !withV {
						amiVersion = strings.TrimPrefix(amiVersion, "v")
					} else if withV && !strings.HasPrefix(amiVersion, "v") {
						amiVersion = "v" + amiVersion
					}
					alias := fmt.Sprintf("%s@%s", amiFamily, amiVersion)
					pod := coretest.Pod()
					NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
					Env.ExpectCreated(NodeClass, NodePool, pod)
					Env.EventuallyExpectHealthy(pod)
					Env.ExpectCreatedNodeCount("==", 1)
				},
				Entry("AL2023 (pinned)", "al2023", true),
				Entry("AL2 (pinned)", "al2", true),
				Entry("Bottlerocket (pinned with v prefix)", "bottlerocket", true),
				Entry("Bottlerocket (pinned without v prefix)", "bottlerocket", false),
			)
			It("should support Custom AMIFamily with AMI Selectors", func() {
				al2023AMI := Env.GetAMIBySSMPath(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/x86_64/standard/recommended/image_id", Env.K8sVersion()))
				NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: al2023AMI}}
				rawContent, err := testdata.ReadFile("testdata/al2023_userdata_input.yaml")
				Expect(err).ToNot(HaveOccurred())
				NodeClass.Spec.UserData = lo.ToPtr(fmt.Sprintf(string(rawContent), Env.ClusterName,
					Env.ClusterEndpoint, Env.ExpectCABundle()))
				pod := coretest.Pod()

				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)

				Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("ImageId", HaveValue(Equal(al2023AMI))))
			})
			It("should have the EC2NodeClass status for AMIs using wildcard", func() {
				NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
					{
						Name: "*",
					},
				}
				Env.ExpectCreated(NodeClass)
				nc := EventuallyExpectAMIsToExist(NodeClass)
				Expect(len(nc.Status.AMIs)).To(BeNumerically("<", 10))
			})
			It("should have the EC2NodeClass status for AMIs using tags", func() {
				NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: customAMI}}
				Env.ExpectCreated(NodeClass)
				nc := EventuallyExpectAMIsToExist(NodeClass)
				Expect(len(nc.Status.AMIs)).To(BeNumerically("==", 1))
				Expect(nc.Status.AMIs[0].ID).To(Equal(customAMI))
				ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeAMIsReady, Status: metav1.ConditionTrue})
				ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
			})
			It("should have the EC2NodeClass status for AMIs using public ssm parameter ARN", func() {
				NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{SSMParameter: fmt.Sprintf("arn:aws:ssm:%s::parameter%s", Env.Region, ssmPath)}}
				Env.ExpectCreated(NodeClass)
				nc := EventuallyExpectAMIsToExist(NodeClass)
				Expect(len(nc.Status.AMIs)).To(BeNumerically("==", 1))
				Expect(nc.Status.AMIs[0].ID).To(Equal(customAMI))
				ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeAMIsReady, Status: metav1.ConditionTrue})
				ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
			})
			It("should have ec2nodeClass status as not ready since AMI was not resolved", func() {
				NodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-123"}}
				Env.ExpectCreated(NodeClass)
				ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeAMIsReady, Status: metav1.ConditionFalse, Message: "AMISelector did not match any AMIs"})
				ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False, AMIsReady=False"})

			})
		})

		Context("UserData", func() {
			It("should merge UserData contents for AL2 AMIFamily", func() {
				if Env.K8sMinorVersion() > 32 {
					Skip("AL2 is not supported on versions > 1.32")
				}
				content, err := testdata.ReadFile("testdata/al2_userdata_input.sh")
				Expect(err).ToNot(HaveOccurred())
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
				NodeClass.Spec.UserData = awssdk.String(string(content))
				NodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
				NodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}
				pod := coretest.Pod(coretest.PodOptions{Tolerations: []corev1.Toleration{{Key: "example.com", Operator: corev1.TolerationOpExists}}})

				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthy(pod)
				Expect(Env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
				))
				actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
				Expect(err).ToNot(HaveOccurred())
				// Since the node has joined the cluster, we know our bootstrapping was correct.
				// Just verify if the UserData contains our custom content too, rather than doing a byte-wise comparison.
				Expect(string(actualUserData)).To(ContainSubstring("Running custom user data script"))
				Expect(string(actualUserData)).To(ContainSubstring("karpenter.sh/do-not-sync-taints=true"))
			})
			It("should merge non-MIME UserData contents for AL2 AMIFamily", func() {
				if Env.K8sMinorVersion() > 32 {
					Skip("AL2 is not supported on versions > 1.32")
				}
				content, err := testdata.ReadFile("testdata/al2_no_mime_userdata_input.sh")
				Expect(err).ToNot(HaveOccurred())
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
				NodeClass.Spec.UserData = awssdk.String(string(content))
				NodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
				NodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}
				pod := coretest.Pod(coretest.PodOptions{Tolerations: []corev1.Toleration{{Key: "example.com", Operator: corev1.TolerationOpExists}}})

				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthy(pod)
				Expect(Env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
				))
				actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
				Expect(err).ToNot(HaveOccurred())
				// Since the node has joined the cluster, we know our bootstrapping was correct.
				// Just verify if the UserData contains our custom content too, rather than doing a byte-wise comparison.
				Expect(string(actualUserData)).To(ContainSubstring("Running custom user data script"))
				Expect(string(actualUserData)).To(ContainSubstring("karpenter.sh/do-not-sync-taints=true"))
			})
			It("should merge UserData contents for Bottlerocket AMIFamily", func() {
				content, err := testdata.ReadFile("testdata/br_userdata_input.sh")
				Expect(err).ToNot(HaveOccurred())
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
				NodeClass.Spec.UserData = awssdk.String(string(content))
				NodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
				NodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}
				pod := coretest.Pod(coretest.PodOptions{Tolerations: []corev1.Toleration{{Key: "example.com", Operator: corev1.TolerationOpExists}}})

				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthy(pod)
				Expect(Env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
				))
				actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(actualUserData)).To(ContainSubstring("kube-api-qps = 30"))
				Expect(string(actualUserData)).To(ContainSubstring("'karpenter.sh/do-not-sync-taints' = 'true'"))
				Expect(string(actualUserData)).To(ContainSubstring("eviction-max-pod-grace-period = 40"))
				Expect(string(actualUserData)).To(ContainSubstring("[settings.kubernetes.eviction-soft]\n'memory.available' = '100Mi'"))
				Expect(string(actualUserData)).To(ContainSubstring("[settings.kubernetes.eviction-soft-grace-period]\n'memory.available' = '30s'"))
			})
			// Windows tests are can flake due to the instance types that are used in testing.
			// The VPC Resource controller will need to support the instance types that are used.
			// If the instance type is not supported by the controller resource `vpc.amazonaws.com/PrivateIPv4Address` will not register.
			// Issue: https://github.com/aws/karpenter-provider-aws/issues/4472
			// See: https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/master/pkg/aws/vpc/limits.go
			It("should merge UserData contents for Windows AMIFamily", func() {
				Env.ExpectWindowsIPAMEnabled()
				DeferCleanup(func() {
					Env.ExpectWindowsIPAMDisabled()
				})

				content, err := testdata.ReadFile("testdata/windows_userdata_input.ps1")
				Expect(err).ToNot(HaveOccurred())
				NodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
				NodeClass.Spec.UserData = awssdk.String(string(content))
				NodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoExecute"}}
				NodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "example.com", Value: "value", Effect: "NoSchedule"}}

				NodePool = coretest.ReplaceRequirements(NodePool,
					karpv1.NodeSelectorRequirementWithMinValues{
						Key:      corev1.LabelOSStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{string(corev1.Windows)},
					},
				)
				pod := coretest.Pod(coretest.PodOptions{
					Image: environmentaws.WindowsDefaultImage,
					NodeSelector: map[string]string{
						corev1.LabelOSStable:     string(corev1.Windows),
						corev1.LabelWindowsBuild: "10.0.20348",
					},
					Tolerations: []corev1.Toleration{{Key: "example.com", Operator: corev1.TolerationOpExists}},
				})

				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthyWithTimeout(time.Minute*15, pod) // Wait 15 minutes because Windows nodes/containers take longer to spin up
				Expect(Env.GetNode(pod.Spec.NodeName).Spec.Taints).To(ContainElements(
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoExecute"},
					corev1.Taint{Key: "example.com", Value: "value", Effect: "NoSchedule"},
				))
				actualUserData, err := base64.StdEncoding.DecodeString(*getInstanceAttribute(pod.Spec.NodeName, "userData").UserData.Value)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(actualUserData)).To(ContainSubstring("Write-Host \"Running custom user data script\""))
				Expect(string(actualUserData)).To(ContainSubstring("[string]$EKSBootstrapScriptFile = \"$env:ProgramFiles\\Amazon\\EKS\\Start-EKSBootstrap.ps1\""))
				Expect(string(actualUserData)).To(ContainSubstring("karpenter.sh/do-not-sync-taints=true"))
			})
		})
	})
	return true
}

//nolint:unparam
func getInstanceAttribute(nodeName string, attribute string) *ec2.DescribeInstanceAttributeOutput {
	var node corev1.Node
	Expect(Env.Client.Get(Env.Context, types.NamespacedName{Name: nodeName}, &node)).To(Succeed())
	providerIDSplit := strings.Split(node.Spec.ProviderID, "/")
	instanceID := providerIDSplit[len(providerIDSplit)-1]
	instanceAttribute, err := Env.EC2API.DescribeInstanceAttribute(Env.Context, &ec2.DescribeInstanceAttributeInput{
		InstanceId: awssdk.String(instanceID),
		Attribute:  ec2types.InstanceAttributeName(attribute),
	})
	Expect(err).ToNot(HaveOccurred())
	return instanceAttribute
}

func EventuallyExpectAMIsToExist(nodeClass *v1.EC2NodeClass) *v1.EC2NodeClass {
	nc := &v1.EC2NodeClass{}
	Eventually(func(g Gomega) {
		g.Expect(Env.Client.Get(Env, client.ObjectKeyFromObject(nodeClass), nc)).To(Succeed())
		g.Expect(nc.Status.AMIs).ToNot(BeNil())
	}).WithTimeout(60 * time.Second).Should(Succeed())
	return nc
}
