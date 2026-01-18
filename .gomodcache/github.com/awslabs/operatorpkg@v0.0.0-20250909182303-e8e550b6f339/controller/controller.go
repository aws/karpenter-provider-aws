package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Controller is a reconciler that allows registration with a controller-runtime Manager
type Controller interface {
	Register(context.Context, manager.Manager) error
}
