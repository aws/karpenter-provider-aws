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

package chaos_test

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	fistypes "github.com/aws/aws-sdk-go-v2/service/fis/types"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/google/uuid"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	awsenv "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// FIS role variables are defined at package level to be accessible across tests
var fisRoleName string
var fisRoleArn string

var _ = Describe("ZonalFailure", func() {
	BeforeEach(func() {
		setupFISRole(env)
	})

	AfterEach(func() {
		cleanupFISRole(env)
	})

	DescribeTable("should recover from AZ failures with different failure rates",
		func(failureRate string, appLabel string, description string) {
			By(fmt.Sprintf("Creating a multi-AZ deployment with multiple replicas for %s test", description))
			nodePool = coretest.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeSpot},
				},
			})
			nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
			nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("30s")

			numPods := 15
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": appLabel},
					},
					TerminationGracePeriodSeconds: lo.ToPtr(int64(0)),
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       "topology.kubernetes.io/zone",
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": appLabel},
							},
						},
					},
				},
			})
			selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

			// Create a deployment with multiple pods
			env.ExpectCreated(nodeClass, nodePool, dep)

			// Wait for all pods to be running
			env.EventuallyExpectHealthyPodCount(selector, numPods)

			// Start a context with a timeout for the chaos test
			ctx, cancel := context.WithTimeout(env.Context, 15*time.Minute)
			defer cancel()

			// Start node count monitor
			startNodeCountMonitor(ctx, env.Client)

			// Get current nodes and group them by AZ
			nodesByAZ := make(map[string][]*corev1.Node)
			nodeList := &corev1.NodeList{}
			Expect(env.Client.List(ctx, nodeList, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())

			// Process nodes, find AZ with most nodes, and collect instances in a single pass
			var targetAZ string
			var maxNodes int
			var instances []ec2types.Instance

			for i := range nodeList.Items {
				node := &nodeList.Items[i]
				az := node.Labels[corev1.LabelTopologyZone]
				if az == "" {
					continue
				}
				nodesByAZ[az] = append(nodesByAZ[az], node)
				if len(nodesByAZ[az]) > maxNodes {
					maxNodes = len(nodesByAZ[az])
					targetAZ = az
				}
			}

			// Ensure we have nodes in multiple AZs
			Expect(len(nodesByAZ)).To(BeNumerically(">", 1), "Expected nodes in multiple AZs")

			By(fmt.Sprintf("Simulating %s with %d nodes (%s%% failure rate)",
				description, maxNodes, failureRate))

			// Get EC2 instance information for nodes in the target AZ
			for _, node := range nodesByAZ[targetAZ] {
				instance := env.GetInstance(node.Name)
				instances = append(instances, instance)
			}

			// Create FIS experiment to simulate AZ failure with specified failure rate
			experiment := createAZFailureExperiment(ctx, env, targetAZ, instances, failureRate)

			// Wait for the experiment to complete
			By(fmt.Sprintf("Waiting for the %s experiment to complete", description))
			Eventually(func(g Gomega) {
				select {
				case <-ctx.Done():
					By("Chaos test timeout reached, skipping experiment status check")
					return
				default:
					exp, err := env.FISAPI.GetExperiment(ctx, &fis.GetExperimentInput{
						Id: experiment.Id,
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(exp.Experiment.State.Status).To(Or(
						Equal(fistypes.ExperimentStatusCompleted),
						Equal(fistypes.ExperimentStatusStopped),
						Equal(fistypes.ExperimentStatusFailed),
					))
				}
			}, 10*time.Minute, 30*time.Second).Should(Succeed())

			// Verify that the system recovered
			By(fmt.Sprintf("Verifying system recovery from %s", description))
			env.EventuallyExpectHealthyPodCountWithTimeout(5*time.Minute, selector, numPods)

			// Clean up
			env.ExpectDeleted(dep)
			env.ExpectExperimentTemplateDeleted(*experiment.Id)

			Eventually(func(g Gomega) {
				// First delete all nodes to trigger proper cleanup
				nodeList := &corev1.NodeList{}
				g.Expect(env.Client.List(env.Context, nodeList, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())

				// Delete each node to trigger proper Kubernetes cleanup
				for _, node := range nodeList.Items {
					fmt.Printf("Deleting node %s\n", node.Name)
					g.Expect(env.Client.Delete(env.Context, &node)).To(Succeed())
				}

				// Wait for nodes to be deleted from Kubernetes
				g.Eventually(func() int {
					tempNodeList := &corev1.NodeList{}
					g.Expect(env.Client.List(env.Context, tempNodeList, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())
					return len(tempNodeList.Items)
				}, 2*time.Minute, 10*time.Second).Should(BeZero())

				// Now terminate any remaining EC2 instances to ensure complete cleanup
				describeInstancesOutput, err := env.EC2API.DescribeInstances(env.Context, &ec2.DescribeInstancesInput{
					Filters: []ec2types.Filter{
						{
							Name:   aws.String(fmt.Sprintf("tag:%s", coretest.DiscoveryLabel)),
							Values: []string{env.ClusterName},
						},
					},
				})
				g.Expect(err).NotTo(HaveOccurred())

				// Collect all instance IDs
				var instanceIDs []string
				for _, reservation := range describeInstancesOutput.Reservations {
					for _, instance := range reservation.Instances {
						if instance.InstanceId != nil {
							instanceIDs = append(instanceIDs, *instance.InstanceId)
						}
					}
				}

				// Terminate instances in batches if needed
				if len(instanceIDs) > 0 {
					fmt.Printf("Terminating %d remaining EC2 instances\n", len(instanceIDs))
					_, err := env.EC2API.TerminateInstances(env.Context, &ec2.TerminateInstancesInput{
						InstanceIds: instanceIDs,
					})
					g.Expect(awserrors.IgnoreNotFound(err)).NotTo(HaveOccurred())
				}
			}, 5*time.Minute).Should(Succeed())
		},
		Entry("complete failure", "100", "complete-failure-app", "complete AZ failure"),
		Entry("moderate failure", "50", "moderate-failure-app", "moderate AZ failures"),
		Entry("minor failure", "25", "minor-failure-app", "minor AZ failures"),
	)
})

// createAZFailureExperiment creates an AWS FIS experiment with a specific failure percentage
func createAZFailureExperiment(ctx context.Context, env *awsenv.Environment, targetAZ string, instances []ec2types.Instance, failurePercentage string) *fistypes.Experiment {
	// Filter instances to only include those in the target AZ
	var targetInstances []string
	for _, instance := range instances {
		if instance.InstanceId != nil && instance.Placement != nil && instance.Placement.AvailabilityZone != nil {
			targetInstances = append(targetInstances, *instance.InstanceId)
		}
	}

	// Create experiment template
	template := &fis.CreateExperimentTemplateInput{
		Actions: map[string]fistypes.CreateExperimentTemplateActionInput{
			"stop-instances": {
				ActionId: aws.String("aws:ec2:stop-instances"),
				Parameters: map[string]string{
					"startInstancesAfterDuration": "PT5M", // Start instances after 5 minutes
				},
				Targets: map[string]string{
					"Instances": "target-instances",
				},
			},
			"ec2-capacity-error": {
				ActionId: aws.String("aws:ec2:api-insufficient-instance-capacity-error"),
				Parameters: map[string]string{
					"availabilityZoneIdentifiers": targetAZ,
					"duration":                    "PT5M",            // 5 minutes
					"percentage":                  failurePercentage, // Percentage of API calls that will fail
				},
				Targets: map[string]string{
					"Roles": "target-roles",
				},
			},
			"disrupt-subnet": {
				ActionId: aws.String("aws:network:disrupt-connectivity"),
				Parameters: map[string]string{
					"duration": "PT5M", // 5 minutes
					"scope":    "all",
				},
				Targets: map[string]string{
					"Subnets": "target-subnets",
				},
			},
		},
		Targets: map[string]fistypes.CreateExperimentTemplateTargetInput{
			"target-instances": {
				ResourceType:  aws.String("aws:ec2:instance"),
				SelectionMode: aws.String("all"),
				ResourceArns: lo.Map(targetInstances, func(id string, _ int) string {
					return fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", env.Region, env.ExpectAccountID(), id)
				}),
			},
			"target-roles": {
				ResourceType:  aws.String("aws:iam:role"),
				SelectionMode: aws.String("all"),
				ResourceArns: []string{
					fmt.Sprintf("arn:aws:iam::%s:role/KarpenterNodeRole-%s", env.ExpectAccountID(), env.ClusterName),
				},
			},
			"target-subnets": {
				ResourceType:  aws.String("aws:ec2:subnet"),
				SelectionMode: aws.String("all"),
				Filters: []fistypes.ExperimentTemplateTargetInputFilter{
					{
						Path: aws.String("AvailabilityZone"),
						Values: []string{
							targetAZ,
						},
					},
				},
			},
		},
		StopConditions: []fistypes.CreateExperimentTemplateStopConditionInput{
			{
				Source: aws.String("none"),
			},
		},
		RoleArn:     aws.String(fisRoleArn),
		Description: aws.String(fmt.Sprintf("Simulate AZ failure in %s", targetAZ)),
	}

	// Create experiment template
	experimentTemplate, err := env.FISAPI.CreateExperimentTemplate(ctx, template)
	Expect(err).NotTo(HaveOccurred())

	// Start experiment
	experiment, err := env.FISAPI.StartExperiment(ctx, &fis.StartExperimentInput{
		ExperimentTemplateId: experimentTemplate.ExperimentTemplate.Id,
	})
	Expect(err).NotTo(HaveOccurred())

	return experiment.Experiment
}

// setupFISRole creates a role for AWS FIS with necessary permissions
func setupFISRole(env *awsenv.Environment) {
	// Create a unique role name for this test run to avoid conflicts
	uid, err := uuid.NewUUID()
	Expect(err).NotTo(HaveOccurred())
	fisRoleName = fmt.Sprintf("KarpenterFISZonalFailureRole-%s", uid.String())

	// Create the FIS role with necessary permissions
	By("Creating FIS role for zonal failure testing")
	assumeRolePolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "fis.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`

	createRoleOutput, err := env.IAMAPI.CreateRole(env.Context, &awsiam.CreateRoleInput{
		RoleName:                 aws.String(fisRoleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
		Description:              aws.String("Role for Karpenter zonal failure testing with AWS FIS"),
		Tags: []iamtypes.Tag{
			{
				Key:   aws.String(coretest.DiscoveryLabel),
				Value: aws.String(env.ClusterName),
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())
	fisRoleArn = *createRoleOutput.Role.Arn

	// Create policy with necessary permissions for FIS actions
	uid, err = uuid.NewUUID()
	Expect(err).NotTo(HaveOccurred())
	policyName := fmt.Sprintf("KarpenterFISZonalFailurePolicy-%s", uid.String())
	policyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"ec2:DescribeInstances",
					"ec2:StopInstances",
					"ec2:StartInstances"
				],
				"Resource": "*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"ec2:CreateTags"
				],
				"Resource": "arn:aws:ec2:*:*:instance/*",
				"Condition": {
					"StringEquals": {
						"ec2:CreateAction": "StopInstances"
					}
				}
			},
			{
				"Effect": "Allow",
				"Action": [
					"network-manager:*",
					"ec2:*NetworkInsightsAccessScope*",
					"ec2:*NetworkInsightsAnalysis*",
					"ec2:*NetworkInsightsPath*"
				],
				"Resource": "*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"iam:GetRole",
					"iam:ListRoles"
				],
				"Resource": "*"
			}
		]
	}`

	createPolicyOutput, err := env.IAMAPI.CreatePolicy(env.Context, &awsiam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
		Description:    aws.String("Policy for Karpenter zonal failure testing with AWS FIS"),
		Tags: []iamtypes.Tag{
			{
				Key:   aws.String(coretest.DiscoveryLabel),
				Value: aws.String(env.ClusterName),
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	// Attach policy to role
	_, err = env.IAMAPI.AttachRolePolicy(env.Context, &awsiam.AttachRolePolicyInput{
		RoleName:  aws.String(fisRoleName),
		PolicyArn: createPolicyOutput.Policy.Arn,
	})
	Expect(err).NotTo(HaveOccurred())

	// Wait for role to propagate
	time.Sleep(10 * time.Second)
}

// cleanupFISRole removes the FIS role and associated policies
func cleanupFISRole(env *awsenv.Environment) {
	// Clean up the FIS role and policy
	By("Cleaning up FIS role and policy")

	// List attached policies
	listPoliciesOutput, err := env.IAMAPI.ListAttachedRolePolicies(env.Context, &awsiam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(fisRoleName),
	})
	if err == nil {
		// Detach and delete policies
		for _, policy := range listPoliciesOutput.AttachedPolicies {
			_, err = env.IAMAPI.DetachRolePolicy(env.Context, &awsiam.DetachRolePolicyInput{
				RoleName:  aws.String(fisRoleName),
				PolicyArn: policy.PolicyArn,
			})
			if err == nil {
				// Delete policy
				_, _ = env.IAMAPI.DeletePolicy(env.Context, &awsiam.DeletePolicyInput{
					PolicyArn: policy.PolicyArn,
				})
			}
		}
	}

	// Delete role
	_, _ = env.IAMAPI.DeleteRole(env.Context, &awsiam.DeleteRoleInput{
		RoleName: aws.String(fisRoleName),
	})
}
