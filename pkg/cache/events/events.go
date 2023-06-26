package events

import (
	"fmt"

	"github.com/aws/karpenter-core/pkg/events"
	v1 "k8s.io/api/core/v1"
)

func UnavailableOfferingEvent(instanceType, availabilityZone, capacityType string) events.Event {
	message := fmt.Sprintf("InsufficientCapacityError for the instanceType %s, availabilityZone %s, capacityType %s", instanceType, availabilityZone, capacityType)
	dedupeValues := []string{fmt.Sprintf("ice-%s-%s-%s", instanceType, availabilityZone, capacityType)}

	return events.Event{
		Type:         v1.EventTypeWarning,
		Reason:       "InsufficientCapacityError",
		Message:      message,
		DedupeValues: dedupeValues,
	}
}
