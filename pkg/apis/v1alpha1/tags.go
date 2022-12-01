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
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/injection"
)

func MergeTags(ctx context.Context, custom ...map[string]string) (result []*ec2.Tag) {
	tags := map[string]string{
		v1alpha5.ProvisionerNameLabelKey: injection.GetNamespacedName(ctx).Name,
		"Name":                           fmt.Sprintf("%s/%s", v1alpha5.ProvisionerNameLabelKey, injection.GetNamespacedName(ctx).Name),
	}
	// Custom tags may override defaults (e.g. Name)
	for _, t := range custom {
		tags = lo.Assign(tags, t)
	}
	for key, value := range tags {
		result = append(result, &ec2.Tag{Key: aws.String(key), Value: aws.String(value)})
	}
	return result
}
