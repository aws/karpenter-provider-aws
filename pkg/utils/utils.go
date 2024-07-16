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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var (
	instanceIDRegex = regexp.MustCompile(`aws:///(?P<AZ>.*)/(?P<InstanceID>.*)`)
)

// ParseInstanceID parses the provider ID stored on the node to get the instance ID
// associated with a node
func ParseInstanceID(providerID string) (string, error) {
	matches := instanceIDRegex.FindStringSubmatch(providerID)
	if matches == nil {
		return "", fmt.Errorf("parsing instance id %s", providerID)
	}
	for i, name := range instanceIDRegex.SubexpNames() {
		if name == "InstanceID" {
			return matches[i], nil
		}
	}
	return "", fmt.Errorf("parsing instance id %s", providerID)
}

// MergeTags takes a variadic list of maps and merges them together into a list of
// EC2 tags to be passed into EC2 API calls
func MergeTags(tags ...map[string]string) []*ec2.Tag {
	return lo.MapToSlice(lo.Assign(tags...), func(k, v string) *ec2.Tag {
		return &ec2.Tag{Key: aws.String(k), Value: aws.String(v)}
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

// GetKubletConfigurationWithNodePool use the most recent version of the kubelet configuration.
// The priority of fields is listed below:
// 1.) v1 NodePool kubelet annotation (Showing a user configured using v1beta1 NodePool at some point)
// 2.) v1 EC2NodeClass will be used (showing a user configured using v1 EC2NodeClass)
func GetKubletConfigurationWithNodePool(nodePool *karpv1.NodePool, nodeClass *v1.EC2NodeClass) (*v1.KubeletConfiguration, error) {
	if annotation, ok := nodePool.Annotations[karpv1.KubeletCompatabilityAnnotationKey]; ok {
		return parseKubeletConfiguration(annotation)
	}
	return nodeClass.Spec.Kubelet, nil
}

func GetKubeletConfigurationWithNodeClaim(nodeClaim *karpv1.NodeClaim, nodeClass *v1.EC2NodeClass) (*v1.KubeletConfiguration, error) {
	if annotation, ok := nodeClaim.Annotations[karpv1.KubeletCompatabilityAnnotationKey]; ok {
		return parseKubeletConfiguration(annotation)
	}
	return nodeClass.Spec.Kubelet, nil
}

func parseKubeletConfiguration(annotation string) (*v1.KubeletConfiguration, error) {
	kubelet := &karpv1beta1.KubeletConfiguration{}
	err := json.Unmarshal([]byte(annotation), kubelet)
	if err != nil {
		return nil, fmt.Errorf("parsing kubelet config from %s annotation, %w", karpv1.KubeletCompatabilityAnnotationKey, err)
	}
	return &v1.KubeletConfiguration{
		ClusterDNS:                  kubelet.ClusterDNS,
		MaxPods:                     kubelet.MaxPods,
		PodsPerCore:                 kubelet.PodsPerCore,
		SystemReserved:              kubelet.SystemReserved,
		KubeReserved:                kubelet.KubeReserved,
		EvictionSoft:                kubelet.EvictionSoft,
		EvictionHard:                kubelet.EvictionHard,
		EvictionSoftGracePeriod:     kubelet.EvictionSoftGracePeriod,
		EvictionMaxPodGracePeriod:   kubelet.EvictionMaxPodGracePeriod,
		ImageGCHighThresholdPercent: kubelet.ImageGCHighThresholdPercent,
		ImageGCLowThresholdPercent:  kubelet.ImageGCLowThresholdPercent,
		CPUCFSQuota:                 kubelet.CPUCFSQuota,
	}, nil
}
