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

package fake

import (
	"context"

	"github.com/awslabs/operatorpkg/status"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/test"
)

const (
	DefaultRegion  = "us-west-2"
	DefaultAccount = "123456789"
)

var _ corecloudprovider.CloudProvider = (*CloudProvider)(nil)

type CloudProvider struct {
	InstanceTypes []*corecloudprovider.InstanceType
	ValidAMIs     []string
}

func (c *CloudProvider) Create(_ context.Context, _ *corev1.NodeClaim) (*corev1.NodeClaim, error) {
	name := test.RandomName()
	return &corev1.NodeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeClaimStatus{
			ProviderID: RandomProviderID(),
		},
	}, nil
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context, _ *corev1.NodePool) ([]*corecloudprovider.InstanceType, error) {
	if c.InstanceTypes != nil {
		return c.InstanceTypes, nil
	}
	return []*corecloudprovider.InstanceType{
		{Name: "default-instance-type"},
	}, nil
}

func (c *CloudProvider) IsDrifted(_ context.Context, nodeClaim *corev1.NodeClaim) (corecloudprovider.DriftReason, error) {
	return "drifted", nil
}

func (c *CloudProvider) Get(context.Context, string) (*corev1.NodeClaim, error) {
	return nil, nil
}

func (c *CloudProvider) List(context.Context) ([]*corev1.NodeClaim, error) {
	return nil, nil
}

func (c *CloudProvider) Delete(context.Context, *corev1.NodeClaim) error {
	return nil
}

// Name returns the CloudProvider implementation name.
func (c *CloudProvider) Name() string {
	return "fake"
}

func (c *CloudProvider) GetSupportedNodeClasses() []status.Object {
	return []status.Object{&v1.EC2NodeClass{}}
}
