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
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"regexp"
	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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

// ConvertedKubelet use the most recent version of the kubelet configuration.
// The priority of fields is listed below:
// 1.) v1 NodePool kubelet annotation (Showing a user configured using v1beta1 NodePool at some point)
// 2.) v1 EC2NodeClass will be used (showing a user configured using v1 EC2NodeClass)
func GetKubelet(kubeletAnnotation string, enc *v1.EC2NodeClass) (*v1.KubeletConfiguration, error) {
	if kubeletAnnotation != "" {
		kubelet := &v1beta1.KubeletConfiguration{}
		err := json.Unmarshal([]byte(kubeletAnnotation), kubelet)
		if err != nil {
			return nil, err
		}
		return updateKubeletType(kubelet), nil
	}

	return enc.Spec.Kubelet, nil
}

func updateKubeletType(kubelet *v1beta1.KubeletConfiguration) *v1.KubeletConfiguration {
	resultKubelet := &v1.KubeletConfiguration{}

	resultKubelet.ClusterDNS = kubelet.ClusterDNS
	resultKubelet.MaxPods = kubelet.MaxPods
	resultKubelet.PodsPerCore = kubelet.PodsPerCore
	resultKubelet.SystemReserved = kubelet.SystemReserved
	resultKubelet.KubeReserved = kubelet.KubeReserved
	resultKubelet.EvictionSoft = kubelet.EvictionSoft
	resultKubelet.EvictionHard = kubelet.EvictionHard
	resultKubelet.EvictionSoftGracePeriod = kubelet.EvictionSoftGracePeriod
	resultKubelet.EvictionMaxPodGracePeriod = kubelet.EvictionMaxPodGracePeriod
	resultKubelet.ImageGCHighThresholdPercent = kubelet.ImageGCHighThresholdPercent
	resultKubelet.ImageGCLowThresholdPercent = kubelet.ImageGCLowThresholdPercent
	resultKubelet.CPUCFSQuota = kubelet.CPUCFSQuota

	return resultKubelet
}
