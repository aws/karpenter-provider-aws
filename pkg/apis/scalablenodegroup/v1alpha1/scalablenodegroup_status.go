package v1alpha1

// ScalableNodeGroupStatus holds status information for the ScalableNodeGroup
type ScalableNodeGroupStatus struct {
	// Replicas displays the current size of the ScalableNodeGroup
	Replicas int32 `json:"replicas,omitempty"`
}
