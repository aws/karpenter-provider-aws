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

package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultNodeRoleName       = "KarpenterNodeRole"
	DefaultLaunchTemplateName = "KarpenterLaunchTemplate"
)

type Initialization struct {
	InstanceRole    *iam.Role
	InstanceProfile *iam.InstanceProfile
	LaunchTemplate  *ec2.LaunchTemplate
	Cluster         *eks.Cluster
	ZonalSubnets    map[string]*ec2.Subnet
}

func NewInitialization(EC2API ec2iface.EC2API, EKSAPI eksiface.EKSAPI, IAMAPI iamiface.IAMAPI, config *rest.Config) *Initialization {
	zap.S().Infof("Initializing AWS Cloud Provider")
	cluster := clusterOrDie(EKSAPI, "etarn-dev")
	instanceRole := instanceRoleOrDie(IAMAPI, DefaultNodeRoleName)
	ensureAWSAuthOrDie(config, instanceRole)
	initialization := &Initialization{
		Cluster:         cluster,
		InstanceRole:    instanceRole,
		InstanceProfile: nodeInstanceProfileOrDie(IAMAPI, *instanceRole.RoleName),
		LaunchTemplate:  launchTemplateOrDie(EC2API, cluster, DefaultLaunchTemplateName, *instanceRole.RoleName),
		ZonalSubnets:    zonalSubnetsOrDie(EC2API, cluster),
	}
	zap.S().Infof("Successfully initialized AWS Cloud Provider")
	return initialization
}

func clusterOrDie(eksAPI eksiface.EKSAPI, name string) *eks.Cluster {
	describeClusterOutput, err := eksAPI.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(name),
	})
	log.PanicIfError(err, "Failed to discover EKS Cluster %s", name)
	zap.S().Infof("Successfully discovered EKS Cluster, %s", name)
	return describeClusterOutput.Cluster
}

func nodeInstanceProfileOrDie(IAMAPI iamiface.IAMAPI, name string) *iam.InstanceProfile {
	// 1. Detect existing InstanceProfile
	getInstanceProfileOutput, err := IAMAPI.GetInstanceProfile(&iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	})
	if err == nil {
		zap.S().Infof("Successfully detected instance profile %s", name)
		return getInstanceProfileOutput.InstanceProfile
	} else if aerr, ok := err.(awserr.Error); ok && aerr.Code() != iam.ErrCodeNoSuchEntityException {
		log.PanicIfError(err, "Failed to retrieve instance profile %s", name)
	}

	// 2. Create InstanceProfile
	createInstanceRoleOutput, err := IAMAPI.CreateInstanceProfile(&iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(name),
	})
	log.PanicIfError(err, "Failed to create instance profile %s", name)
	zap.S().Infof("Successfully created instance profile %s", name)

	// 3. Attach Role to Instance Profile
	_, err = IAMAPI.AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(name),
		RoleName:            aws.String(name),
	})
	log.PanicIfError(err, "Failed to add role %s to instance profile %s", name, name)
	zap.S().Infof("Successfully added role %s to instance profile %s", name, name)
	return createInstanceRoleOutput.InstanceProfile
}

func instanceRoleOrDie(IAMAPI iamiface.IAMAPI, name string) *iam.Role {
	// 1. Detect existing Role
	getRoleOutput, err := IAMAPI.GetRole(&iam.GetRoleInput{RoleName: aws.String(name)})
	if err == nil {
		zap.S().Infof("Successfully detected role %s", name)
		return getRoleOutput.Role
	} else if aerr, ok := err.(awserr.Error); ok && aerr.Code() != iam.ErrCodeNoSuchEntityException {
		log.PanicIfError(err, "Failed to retrieve iam role %s", name)
	}

	// 2. Create Role
	createRoleOutput, err := IAMAPI.CreateRole(&iam.CreateRoleInput{
		RoleName: aws.String(name),
		AssumeRolePolicyDocument: aws.String(`{
			"Version": "2012-10-17",
			"Statement": [
			  {
				"Effect": "Allow",
				"Action": [
				  "sts:AssumeRole"
				],
				"Principal": {
				  "Service": [
					"ec2.amazonaws.com"
				  ]
				}
			  }
			]
		  }`),
	})
	log.PanicIfError(err, "Failed to create role %s", name)
	zap.S().Infof("Successfully created role %s", name)

	// 2. Attach policies to role
	for _, policyArn := range []string{
		"arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore",
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
	} {
		_, err := IAMAPI.AttachRolePolicy(&iam.AttachRolePolicyInput{
			PolicyArn: aws.String(policyArn),
			RoleName:  aws.String(name),
		})
		log.PanicIfError(err, "Failed to attach policy %s to role %s", policyArn, name)
		zap.S().Infof("Successfully attached policy %s to role %s", policyArn, name)
	}
	return createRoleOutput.Role
}

func launchTemplateOrDie(ec2API ec2iface.EC2API, cluster *eks.Cluster, name string, instanceProfileName string) *ec2.LaunchTemplate {
	// 1. Detect existing launch template
	describeLaunchTemplateOutput, err := ec2API.DescribeLaunchTemplates(&ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{aws.String(name)},
	})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() != "InvalidLaunchTemplateName.NotFoundException" {
		log.PanicIfError(aerr, "Failed to retrieve default launch template")
	}
	if len(describeLaunchTemplateOutput.LaunchTemplates) > 0 {
		zap.S().Infof("Successfully detected existing launch template, %s/%s",
			*describeLaunchTemplateOutput.LaunchTemplates[0].LaunchTemplateId,
			*describeLaunchTemplateOutput.LaunchTemplates[0].LaunchTemplateName)
		return describeLaunchTemplateOutput.LaunchTemplates[0]
	}

	// 2. Create Launch Template
	createLaunchTemplateOutput, err := ec2API.CreateLaunchTemplate(&ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(name),
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			IamInstanceProfile: &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: aws.String(instanceProfileName),
			},
			TagSpecifications: []*ec2.LaunchTemplateTagSpecificationRequest{{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{{
					Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", *cluster.Name)),
					Value: aws.String("owned"),
				}},
			}},
			SecurityGroupIds: append(cluster.ResourcesVpcConfig.SecurityGroupIds, cluster.ResourcesVpcConfig.ClusterSecurityGroupId),
			UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`
				#!/bin/bash
				yum install -y https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/linux_amd64/amazon-ssm-agent.rpm
				/etc/eks/bootstrap.sh %s \
					--kubelet-extra-args '--node-labels=karpenter.sh/provisioned=true' \
					--b64-cluster-ca %s \
					--apiserver-endpoint %s`,
				*cluster.Name,
				*cluster.CertificateAuthority.Data,
				*cluster.Endpoint,
			)))),
			ImageId: aws.String("ami-0532808ed453f9ca3"),
		},
	})
	log.PanicIfError(err, "Failed to create default launch template")
	zap.S().Infof("Successfully created default launch template, %s/%s",
		*createLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId,
		*createLaunchTemplateOutput.LaunchTemplate.LaunchTemplateName)
	return createLaunchTemplateOutput.LaunchTemplate
}

func zonalSubnetsOrDie(ec2API ec2iface.EC2API, cluster *eks.Cluster) map[string]*ec2.Subnet {
	describeSubnetOutput, err := ec2API.DescribeSubnets(&ec2.DescribeSubnetsInput{
		SubnetIds: cluster.ResourcesVpcConfig.SubnetIds,
	})
	log.PanicIfError(err, "Failed to describe subnets %v", cluster.ResourcesVpcConfig.SubnetIds)
	zonalSubnetMap := map[string]*ec2.Subnet{}
	// TODO Filter public subnets
	for _, subnet := range describeSubnetOutput.Subnets {
		zonalSubnetMap[*subnet.AvailabilityZone] = subnet
	}

	return zonalSubnetMap
}

func ensureAWSAuthOrDie(config *rest.Config, role *iam.Role) {
	kubeClient, err := client.New(config, client.Options{})
	log.PanicIfError(err, "Failed to instantiate ephemeral kubeClient")

	awsAuth := &v1.ConfigMap{}
	nn := types.NamespacedName{Name: "aws-auth", Namespace: "kube-system"}
	log.PanicIfError(kubeClient.Get(context.TODO(), nn, awsAuth), "Failed to retrieve configmap aws-auth")

	if strings.Contains(awsAuth.Data["mapRoles"], *role.Arn) {
		zap.S().Infof("Successfully detected aws-auth configmap contains roleArn %s", *role.Arn)
		return
	}
	// Since the aws-auth configmap is stringly typed, this specific indentation is critical
	awsAuth.Data["mapRoles"] += fmt.Sprintf(`
- groups:
  - system:bootstrappers
  - system:nodes
  rolearn: %s
  username: system:node:{{EC2PrivateDNSName}}`, *role.Arn)
	log.PanicIfError(kubeClient.Update(context.TODO(), awsAuth), "Failed to update configmap aws-auth")
	zap.S().Infof("Successfully patched configmap aws-auth with roleArn %s", *role.Arn)
}
