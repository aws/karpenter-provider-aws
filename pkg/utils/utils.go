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

package utils

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/awslabs/operatorpkg/serrors"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/samber/lo"
)

var (
	instanceIDRegex = regexp.MustCompile(`aws:///(?P<AZ>.*)/(?P<InstanceID>.*)`)
)

// ParseInstanceID parses the provider ID stored on the node to get the instance ID
// associated with a node
func ParseInstanceID(providerID string) (string, error) {
	matches := instanceIDRegex.FindStringSubmatch(providerID)
	if matches == nil {
		return "", serrors.Wrap(fmt.Errorf("provider id does not match known format"), "provider-id", providerID)
	}
	for i, name := range instanceIDRegex.SubexpNames() {
		if name == "InstanceID" {
			return matches[i], nil
		}
	}
	return "", serrors.Wrap(fmt.Errorf("provider id does not match known format"), "provider-id", providerID)
}

// EC2MergeTags takes a variadic list of maps and merges them together into a list of
// EC2 tags to be passed into EC2 API calls
func EC2MergeTags(tags ...map[string]string) []ec2types.Tag {
	return lo.MapToSlice(lo.Assign(tags...), func(k, v string) ec2types.Tag {
		return ec2types.Tag{Key: aws.String(k), Value: aws.String(v)}
	})
}

// EC2MergeTags takes a variadic list of maps and merges them together into a list of
// EC2 tags to be passed into EC2 API calls
func IAMMergeTags(tags ...map[string]string) []iamtypes.Tag {
	return lo.MapToSlice(lo.Assign(tags...), func(k, v string) iamtypes.Tag {
		return iamtypes.Tag{Key: aws.String(k), Value: aws.String(v)}
	})
}

// PrettySlice truncates a slice after a certain number of max items to ensure
// that the Slice isn't too long
func PrettySlice[T any](s []T, maxItems int) string {
	var sb strings.Builder
	for i, elem := range s {
		if i > maxItems-1 {
			fmt.Fprintf(&sb, " and %d other(s)", len(s)-i)
			break
		} else if i > 0 {
			fmt.Fprint(&sb, ", ")
		}
		fmt.Fprint(&sb, elem)
	}
	return sb.String()
}

// WithDefaultFloat64 returns the float64 value of the supplied environment variable or, if not present,
// the supplied default value. If the float64 conversion fails, returns the default
func WithDefaultFloat64(key string, def float64) float64 {
	val, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return f
}

func GetTags(nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, clusterName string) (map[string]string, error) {
	var invalidTags []string
	for key := range nodeClass.Spec.Tags {
		for _, exp := range v1.RestrictedTagPatterns {
			if exp.MatchString(key) {
				invalidTags = append(invalidTags, key)
				break
			}
		}
	}
	if len(invalidTags) != 0 {
		quotedTags := lo.Map(invalidTags, func(tag string, _ int) string {
			return fmt.Sprintf("%q", tag)
		})
		return nil, serrors.Wrap(fmt.Errorf("tags failed validation requirements"), "tags", strings.Join(quotedTags, ", "))
	}
	staticTags := map[string]string{
		fmt.Sprintf("kubernetes.io/cluster/%s", clusterName): "owned",
		karpv1.NodePoolLabelKey:                              nodeClaim.Labels[karpv1.NodePoolLabelKey],
		v1.EKSClusterNameTagKey:                              clusterName,
		v1.LabelNodeClass:                                    nodeClass.Name,
	}
	return lo.Assign(nodeClass.Spec.Tags, staticTags), nil
}
