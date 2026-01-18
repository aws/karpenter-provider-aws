package cli

import (
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
)

type Command interface {
	Run(log *zap.Logger, opts *GlobalOptions) error
	Flaggy() *flaggy.Subcommand
}
