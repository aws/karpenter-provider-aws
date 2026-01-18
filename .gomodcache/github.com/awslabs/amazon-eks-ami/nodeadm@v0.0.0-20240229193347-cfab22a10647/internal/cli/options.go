package cli

import "github.com/integrii/flaggy"

type GlobalOptions struct {
	ConfigSource    string
	DevelopmentMode bool
}

func NewGlobalOptions() *GlobalOptions {
	opts := GlobalOptions{
		ConfigSource:    "imds://user-data",
		DevelopmentMode: false,
	}
	flaggy.String(&opts.ConfigSource, "c", "config-source", "Source of node configuration. The format is a URI with supported schemes: [imds, file].")
	flaggy.Bool(&opts.DevelopmentMode, "d", "development", "Enable development mode for logging.")
	return &opts
}
