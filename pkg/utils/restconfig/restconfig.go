package restconfig

import (
	"context"

	"k8s.io/client-go/rest"
)

type contextKey string

const (
	restKey contextKey = "restKey"
)

func Inject(ctx context.Context, config *rest.Config) context.Context {
	return context.WithValue(ctx, restKey, config)
}

func Get(ctx context.Context) *rest.Config {
	retval := ctx.Value(restKey)
	if retval == nil {
		return nil
	}
	return retval.(*rest.Config)
}
