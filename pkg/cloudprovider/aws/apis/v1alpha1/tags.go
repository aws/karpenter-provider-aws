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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
)

const (
	// ClusterTagKeyFormat is set on all Kubernetes owned resources.
	ClusterTagKeyFormat = "kubernetes.io/cluster/%s"
	// KarpenterTagKeyFormat is set on all Karpenter owned resources.
	KarpenterTagKeyFormat = "karpenter.sh/cluster/%s"
)

func ManagedTagsFor(clusterName string) map[string]string {
	// tags to be applied on AWS resources created by Karpenter (instances, launchTemplates..)
	return map[string]string{
		fmt.Sprintf(ClusterTagKeyFormat, clusterName):   "owned",
		fmt.Sprintf(KarpenterTagKeyFormat, clusterName): "owned",
	}
}

func MergeTags(ctx context.Context, customTags map[string]string) []*ec2.Tag {
	managedTags := ManagedTagsFor(injection.GetOptions(ctx).ClusterName)
	// We'll set the default Name tag, but allow it to be overridden in the merge
	managedTags["Name"] = fmt.Sprintf("karpenter.sh/cluster/%s/provisioner/%s",
		injection.GetOptions(ctx).ClusterName, injection.GetNamespacedName(ctx).Name)
	ec2Tags := []*ec2.Tag{}
	for key, value := range functional.UnionStringMaps(managedTags, customTags) {
		ec2Tags = append(ec2Tags, &ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	return ec2Tags
}
