package config

import (
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/cli"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/configprovider"
	"github.com/integrii/flaggy"
	"go.uber.org/zap"
)

type fileCmd struct {
	cmd *flaggy.Subcommand
}

func NewCheckCommand() cli.Command {
	cmd := flaggy.NewSubcommand("check")
	cmd.Description = "Verify configuration"
	return &fileCmd{
		cmd: cmd,
	}
}

func (c *fileCmd) Flaggy() *flaggy.Subcommand {
	return c.cmd
}

func (c *fileCmd) Run(log *zap.Logger, opts *cli.GlobalOptions) error {
	log.Info("Checking configuration", zap.String("source", opts.ConfigSource))
	provider, err := configprovider.BuildConfigProvider(opts.ConfigSource)
	if err != nil {
		return err
	}
	_, err = provider.Provide()
	if err != nil {
		return err
	}
	log.Info("Configuration is valid")
	return nil
}
