package bench

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Context holds the shared context for test suites
type Defintion struct {
	Deployment *appsv1.Deployment
	Replicas   int32
}

// Suite defines the interface for test suites
type Suite interface {
	// Run executes the test suite
	Run(context.Context, client.Client, *v1.Deployment) error

	// Description returns a human-readable description of the test suite
	Description() string
}
