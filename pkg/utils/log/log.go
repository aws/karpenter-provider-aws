package log

import (
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func Setup(opts ...controllerruntimezap.Opts) {
	logger := controllerruntimezap.NewRaw(opts...)
	controllerruntime.SetLogger(zapr.NewLogger(logger))
	zap.ReplaceGlobals(logger)
}
