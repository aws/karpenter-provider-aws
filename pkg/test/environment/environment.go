package environment

// Environment encapsulates a testing environment, such as a local API server or kubernetes test cluster.
type Environment interface {
	// Start must be called before the environment is used to setup resources.
	Start() error
	// Stop must be called after the environment is used to clean up resources.
	Stop() error
	// Namespace instantiates a new kubernetes namespace for testing. The namespace
	// will be automatically created when instantiated and deleted during env.Stop().
	NewNamespace() (*Namespace, error)
}
