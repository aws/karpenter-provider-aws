package node

import (
	"strings"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"
)

// If other cloud providers are formatted differently, this will need to be refactored
func GetInstanceId(node *v1.Node) *string {
	id := strings.Split(node.Spec.ProviderID, "/")
	if len(id) < 5 {
		zap.S().Debugf("Could not parse instance ProviderID, %s has invalid format", node.Name)
		return nil
	}
	return ptr.String(id[4])
}
