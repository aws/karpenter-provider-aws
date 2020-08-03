package aws

// AWS implements the CloudProvider interface
type AWS struct {
}

// SetReplicas of the underlying ASG
func (a *AWS) SetReplicas(value int32) error {
	return nil
}

// Node Terminiation: AWS CloudProvider should use LifecycleHooks to ensure that all pods have terminated successfully before allowing node termination.
// 1. Mark Node as Unschedulable
// 2. Evict the pods on the node
