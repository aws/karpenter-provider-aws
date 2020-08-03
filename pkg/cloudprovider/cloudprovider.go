package cloudprovider

// CloudProvider abstracts all provider specific behavior.
type CloudProvider interface {
	// SetReplicas sets the MachineDeployment's replica count
	SetReplicas(value int32)
}
