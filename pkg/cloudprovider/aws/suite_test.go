package aws

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/ellistarn/karpenter/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAWS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"AWS Cloud Provider",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = Describe("ManagedNodeGroup", func() {
	Context(".GetReplicas()", func() {
		It("should return all nodes that are ready and match the label selector", func() {
			mng := &ManagedNodeGroup{
				NodeGroup: "testgroup",
				Client: clientfake.NewFakeClient(
					// Include
					test.NodeWith(test.NodeOptions{Labels: map[string]string{NodeGroupLabel: "testgroup"}}),
					test.NodeWith(test.NodeOptions{Labels: map[string]string{NodeGroupLabel: "testgroup"}}),
					// Exclude: not ready
					test.NodeWith(test.NodeOptions{Labels: map[string]string{NodeGroupLabel: "testgroup"}, ReadyStatus: v1.ConditionFalse}),
					// Exclude: not in node group
					test.NodeWith(test.NodeOptions{}),
				),
			}
			replicas, err := mng.GetReplicas()
			Expect(err).ToNot(HaveOccurred())
			Expect(replicas).To(BeEquivalentTo(2))
		})
	})

	Context(".SetReplicas()", func() {
		It("should set replicas on the underlying managed node group", func() {
			mng := &ManagedNodeGroup{
				EKSClient: fake.EKSAPI{UpdateOutput: eks.UpdateNodegroupConfigOutput{}},
			}
			Expect(mng.SetReplicas(10)).To(Succeed())
		})

		It("should fail if the EKS Client throws an error", func() {
			mng := &ManagedNodeGroup{
				EKSClient: fake.EKSAPI{
					WantErr: errors.New("Failed to upgrade"),
				},
			}
			Expect(mng.SetReplicas(10)).ToNot(Succeed())
		})
	})

	Context(".Stabilized()", func() {
		It("should return false if asg replicas don't match desired replicas", func() {
			Skip("Not yet implemented")
			mng := &ManagedNodeGroup{
				EKSClient: fake.EKSAPI{
					DescribeOutput: eks.DescribeNodegroupOutput{},
				},
				AutoscalingClient: fake.AutoScalingAPI{
					DescribeOutput: autoscaling.DescribeAutoScalingGroupsOutput{},
				},
			}
			stabilized, message, err := mng.Stabilized()
			Expect(stabilized).To(BeTrue())
			Expect(message).To(Equal(""))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return true if asg replicas match desired replicas", func() {
			Skip("Not yet implemented")
			mng := &ManagedNodeGroup{
				EKSClient: fake.EKSAPI{
					DescribeOutput: eks.DescribeNodegroupOutput{},
				},
				AutoscalingClient: fake.AutoScalingAPI{
					DescribeOutput: autoscaling.DescribeAutoScalingGroupsOutput{},
				},
			}
			stabilized, message, err := mng.Stabilized()
			Expect(stabilized).To(BeFalse())
			Expect(message).To(Equal(""))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail if the EKS Client throws an error", func() {
			Skip("Not yet implemented")
			mng := &ManagedNodeGroup{
				EKSClient: fake.EKSAPI{
					WantErr: errors.New("Failed to upgrade"),
				},
			}
			_, _, err := mng.Stabilized()
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("AutoScalingGroup", func() {
	Context(".GetReplicas()", func() {
		It("should return all nodes that are ready and match the label selector", func() {
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					DescribeOutput: autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{Instances: []*autoscaling.Instance{nil, nil, nil}},
						},
					},
				},
			}
			replicas, err := asg.GetReplicas()
			Expect(err).ToNot(HaveOccurred())
			Expect(replicas).To(BeEquivalentTo(3))
		})

		It("should fail if multiple asgs are returned", func() {
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					DescribeOutput: autoscaling.DescribeAutoScalingGroupsOutput{
						AutoScalingGroups: []*autoscaling.Group{
							{Instances: []*autoscaling.Instance{}},
							// Second ASG is invalid
							{Instances: []*autoscaling.Instance{}},
						},
					},
				},
			}
			_, err := asg.GetReplicas()
			Expect(err).To(HaveOccurred())
		})

		It("should fail if the Autoscaling Client throws an error", func() {
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					WantErr: errors.New("Something bad happened"),
				},
			}
			_, err := asg.GetReplicas()
			Expect(err).To(HaveOccurred())
		})
	})

	Context(".SetReplicas()", func() {
		It("should set replicas on the underlying managed node group", func() {
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					UpdateOutput: autoscaling.UpdateAutoScalingGroupOutput{},
				},
			}
			Expect(asg.SetReplicas(3)).To(Succeed())
		})

		It("should fail if the EKS Client throws an error", func() {
			mng := &ManagedNodeGroup{
				EKSClient: fake.EKSAPI{
					WantErr: errors.New("Failed to upgrade"),
				},
			}
			Expect(mng.SetReplicas(10)).ToNot(Succeed())
		})
	})

	Context(".Stabilized()", func() {
		It("should return false if asg replicas don't match desired replicas", func() {
			Skip("Not yet implemented")
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					DescribeOutput: autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []*autoscaling.Group{{
						DesiredCapacity: aws.Int64(1),
						Instances:       []*autoscaling.Instance{},
					}}},
				},
			}
			stabilized, message, err := asg.Stabilized()
			Expect(stabilized).To(BeFalse())
			Expect(message).To(Equal(""))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return true if asg replicas match desired replicas", func() {
			Skip("Not yet implemented")
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					DescribeOutput: autoscaling.DescribeAutoScalingGroupsOutput{AutoScalingGroups: []*autoscaling.Group{{
						DesiredCapacity: aws.Int64(0),
						Instances:       []*autoscaling.Instance{},
					}}},
				},
			}
			stabilized, message, err := asg.Stabilized()
			Expect(stabilized).To(BeTrue())
			Expect(message).To(Equal(""))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail if the EKS Client throws an error", func() {
			Skip("Not yet implemented")
			asg := &AutoScalingGroup{
				ID: "testgroup",
				Client: fake.AutoScalingAPI{
					WantErr: fmt.Errorf("Something bad happened"),
				},
			}
			Expect(asg.Stabilized()).ToNot(Succeed())
		})
	})
})
