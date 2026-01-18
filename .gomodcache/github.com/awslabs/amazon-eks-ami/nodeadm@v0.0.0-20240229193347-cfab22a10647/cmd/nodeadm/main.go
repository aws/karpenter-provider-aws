package main

import (
	"github.com/integrii/flaggy"
	"go.uber.org/zap"

	"github.com/awslabs/amazon-eks-ami/nodeadm/cmd/nodeadm/config"
	initcmd "github.com/awslabs/amazon-eks-ami/nodeadm/cmd/nodeadm/init"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/cli"
)

func main() {
	flaggy.SetName("nodeadm")
	flaggy.SetDescription("From zero to Node faster than you can say Elastic Kubernetes Service")
	flaggy.DefaultParser.AdditionalHelpPrepend = "\nhttp://github.com/awslabs/amazon-eks-ami/nodeadm"
	flaggy.DefaultParser.ShowHelpOnUnexpected = true

	opts := cli.NewGlobalOptions()

	cmds := []cli.Command{
		config.NewConfigCommand(),
		initcmd.NewInitCommand(),
	}

	for _, cmd := range cmds {
		flaggy.AttachSubcommand(cmd.Flaggy(), 1)
	}
	flaggy.Parse()

	log := cli.NewLogger(opts)

	for _, cmd := range cmds {
		if cmd.Flaggy().Used {
			err := cmd.Run(log, opts)
			if err != nil {
				log.Fatal("Command failed", zap.Error(err))
			}
			return
		}
	}
	flaggy.ShowHelpAndExit("No command specified")
}
