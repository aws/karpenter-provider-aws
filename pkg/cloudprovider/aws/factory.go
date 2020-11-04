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
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/fake"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Factory struct {
	AutoscalingClient autoscalingiface.AutoScalingAPI
	SQSClient         sqsiface.SQSAPI
	EKSClient         eksiface.EKSAPI
	Client            client.Client
}

func NewFactory(options cloudprovider.Options) *Factory {
	sess := withRegion(session.Must(session.NewSession()))
	return &Factory{
		AutoscalingClient: autoscaling.New(sess),
		EKSClient:         eks.New(sess),
		SQSClient:         sqs.New(sess),
		Client:            options.Client,
	}
}

func (f *Factory) NodeGroupFor(spec *v1alpha1.ScalableNodeGroupSpec) cloudprovider.NodeGroup {
	switch spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		return NewAutoScalingGroup(spec.ID, f.AutoscalingClient)
	case v1alpha1.AWSEKSNodeGroup:
		return NewManagedNodeGroup(spec.ID, f.EKSClient, f.AutoscalingClient, f.Client)
	default:
		return fake.NewNotImplementedFactory().NodeGroupFor(spec)
	}
}

func (f *Factory) QueueFor(spec *v1alpha1.QueueSpec) cloudprovider.Queue {
	switch spec.Type {
	case v1alpha1.AWSSQSQueueType:
		return NewSQSQueue(spec.ID, f.SQSClient)
	default:
		return fake.NewNotImplementedFactory().QueueFor(spec)
	}
}

func withRegion(sess *session.Session) *session.Session {
	region, err := ec2metadata.New(sess).Region()
	log.PanicIfError(err, "failed to call the metadata server's region API")
	sess.Config.Region = aws.String(region)
	return sess
}
