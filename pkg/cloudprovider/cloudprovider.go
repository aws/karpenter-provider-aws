package cloudprovider

// CloudProvider abstracts all instantiation logic.
type CloudProvider interface {
	// NewNodeGroup returns a new NodeGroup for the CloudProvider
	NewNodeGroup(id NodeGroupIdentifier) (NodeGroup, error)
}

type NodeGroupIdentifier interface {
	GroupName() string
	ClusterName() *string
}

// NodeGroup abstracts all provider specific behavior for NodeGroups.
type NodeGroup interface {
	// Name returns the name of the node group
	Id() NodeGroupIdentifier

	// SetReplicas sets the NodeGroups's replica count
	SetReplicas(value int) error
}
