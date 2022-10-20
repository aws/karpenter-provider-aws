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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/operator/injection"
)

// ParseInstanceID parses the provider ID stored on the node to get the instance ID
// associated with a node
func ParseInstanceID(node *v1.Node) (*string, error) {
	r := regexp.MustCompile(`aws:///(?P<AZ>.*)/(?P<InstanceID>.*)`)
	matches := r.FindStringSubmatch(node.Spec.ProviderID)
	if matches == nil {
		return nil, fmt.Errorf("parsing instance id %s", node.Spec.ProviderID)
	}
	for i, name := range r.SubexpNames() {
		if name == "InstanceID" {
			return ptr.String(matches[i]), nil
		}
	}
	return nil, fmt.Errorf("parsing instance id %s", node.Spec.ProviderID)
}

// GetClusterNameHash gets the SHA256 hex-encoded checksum of the cluster name, truncated at the passed truncatedAt
func GetClusterNameHash(ctx context.Context, truncateAt int) string {
	h := sha256.New()
	h.Write([]byte(injection.GetOptions(ctx).ClusterName))
	checkSum := h.Sum([]byte{})
	return hex.EncodeToString(checkSum)[:truncateAt]
}
