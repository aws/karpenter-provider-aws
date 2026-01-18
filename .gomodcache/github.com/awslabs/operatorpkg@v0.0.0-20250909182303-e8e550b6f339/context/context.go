package context

import (
	"context"
	"reflect"

	"github.com/go-logr/zapr"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Context = context.Context

func New() context.Context {
	ctx := controllerruntime.SetupSignalHandler()
	logger := zapr.NewLogger(lo.Must(zap.NewDevelopment()))
	klog.SetLogger(logger)
	log.SetLogger(logger)
	ctx = Into(ctx, &logger)
	return ctx
}

func Into[T any](parent context.Context, v *T) context.Context {
	return context.WithValue(parent, reflect.TypeOf((*T)(nil)), v)
}

func From[T any](ctx context.Context) *T {
	v, _ := ctx.Value(reflect.TypeOf((*T)(nil))).(*T)
	return v
}
