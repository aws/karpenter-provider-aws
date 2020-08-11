package cloudprovider

// CloudProvider abstracts all instantiation logic.
type CloudProvider interface {
	// NewNodeGroup returns a new NodeGroup for the CloudProvider
	NewNodeGroup(name string) NodeGroup
}

// NodeGroup abstracts all provider specific behavior for NodeGroups.
type NodeGroup interface {
	// Name returns the name of the node group
	Name() string

	// SetReplicas sets the MachineDeployment's replica count
	SetReplicas(value int) error
}
