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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/mock"
	nodegroupaws "github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
	"github.com/ellistarn/karpenter/pkg/utils/log"
)

type Factory struct {
	AutoscalingClient autoscalingiface.AutoScalingAPI
	SQSClient         sqsiface.SQSAPI
}

func NewFactory() *Factory {
	session := withRegion(session.Must(session.NewSession()))
	return &Factory{
		AutoscalingClient: autoscaling.New(session),
		SQSClient:         sqs.New(session),
	}
}

func (f *Factory) NodeGroupFor(spec *v1alpha1.ScalableNodeGroupSpec) cloudprovider.NodeGroup {
	switch spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		return nodegroupaws.NewAutoScalingGroup(spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		return nodegroupaws.NewManagedNodeGroup(spec.ID)
	default:
		return mock.NewNotImplementedFactory().NodeGroupFor(spec)
	}
}

func (f *Factory) QueueFor(spec v1alpha1.QueueSpec) cloudprovider.Queue {
	switch spec.Type {
	case v1alpha1.AWSSQSQueueType:
		return NewSQS(spec.ID, NewFactory().SQSClient)
	default:
		return mock.NewNotImplementedFactory().QueueFor(spec)
	}
}

func withRegion(session *session.Session) *session.Session {
	svc := ec2metadata.New(session, aws.NewConfig())
	region, err := svc.Region()
	if err != nil {
		log.PanicIfError(err, "failed to call the metadata server's region API")
	}
	session.Config.Region = aws.String(region)
	return session
}
