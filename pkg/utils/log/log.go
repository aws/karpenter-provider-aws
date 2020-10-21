package log

import (
	"fmt"

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

func InvariantViolated(reason string) {
	zap.S().Errorf("Invariant violated: %s. Is the validation webhook installed?", reason)
}

func PanicIfError(err error, formatter string, arguments ...interface{}) {
	if err != nil {
		zap.S().Panicf("%s, %w", fmt.Sprintf(formatter, arguments...), err)
	}
}
