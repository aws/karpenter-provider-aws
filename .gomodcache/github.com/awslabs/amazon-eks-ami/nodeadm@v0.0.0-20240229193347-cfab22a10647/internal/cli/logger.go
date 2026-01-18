package cli

import (
	"go.uber.org/zap"
)

func NewLogger(opts *GlobalOptions) *zap.Logger {
	var logger *zap.Logger
	var err error
	if opts.DevelopmentMode {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
	return logger
}
