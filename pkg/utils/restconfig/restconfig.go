package restconfig

import (
	"context"

	"k8s.io/client-go/rest"
)

type contextKey struct{}

func Inject(ctx context.Context, config *rest.Config) context.Context {
	return context.WithValue(ctx, contextKey{}, config)
}

func Get(ctx context.Context) *rest.Config {
	retval := ctx.Value(contextKey{})
	if retval == nil {
		return nil
	}
	return retval.(*rest.Config)
}
