package utils

import (
	"fmt"
	"regexp"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"
)

// ParseProviderID parses the provider ID stored on the node to get the instance ID
// associated with a node
func ParseProviderID(node *v1.Node) (*string, error) {
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
