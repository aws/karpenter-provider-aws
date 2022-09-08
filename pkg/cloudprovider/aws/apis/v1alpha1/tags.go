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
	"github.com/mitchellh/hashstructure/v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/injection"
)

func MergeProviderTags(ctx context.Context, provisionerName string, provider *AWS) map[string]string {
	return MergeTags(provisionerName, provider.Tags, map[string]string{fmt.Sprintf("kubernetes.io/cluster/%s", injection.GetOptions(ctx).ClusterName): "owned"})
}

func MergeTags(provisionerName string, custom ...map[string]string) map[string]string {
	tags := map[string]string{
		v1alpha5.ProvisionerNameLabelKey: provisionerName,
		"Name":                           fmt.Sprintf("%s/%s", v1alpha5.ProvisionerNameLabelKey, provisionerName),
	}
	// Custom tags may override defaults (e.g. Name)
	for _, t := range custom {
		tags = lo.Assign(tags, t)
	}

	return tags
}

func ToEC2Tags(tags map[string]string) (result []*ec2.Tag) {
	for key, value := range tags {
		result = append(result, &ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	return result
}

func GetTagsHash(tags []*ec2.Tag) string {
	currentTagsVersion, _ := hashstructure.Hash(tags, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	return fmt.Sprintf("tags-%d", currentTagsVersion)
}
