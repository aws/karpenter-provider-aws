package cache

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/events"
)

func InsufficientCapacityErrorEvent(instanceType, availabilityZone, capacityType string) events.Event {
	return events.Event{
		Type:         v1.EventTypeWarning,
		Reason:       "InsufficientCapacityError",
		Message:      fmt.Sprintf(`InsufficientCapacityError for {"instanceType": %q, "availabilityZone": %q, "capacityType": %q}`, instanceType, availabilityZone, capacityType),
		DedupeValues: []string{instanceType, availabilityZone, capacityType},
	}
}
