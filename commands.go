package main

import (
	"os"

	"github.com/mitchellh/cli"
	cmdSyncCatalog "github.com/nsone/consul-ns1/subcommand/sync-catalog"
	cmdVersion "github.com/nsone/consul-ns1/subcommand/version"
	"github.com/nsone/consul-ns1/version"
)

// Commands is the mapping of all available consul-ns1 commands.
var Commands map[string]cli.CommandFactory

func init() {
	ui := &cli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr}

	Commands = map[string]cli.CommandFactory{
		"sync-catalog": func() (cli.Command, error) {
			return &cmdSyncCatalog.Command{UI: ui}, nil
		},

		"version": func() (cli.Command, error) {
			return &cmdVersion.Command{UI: ui, Version: version.GetHumanVersion()}, nil
		},
	}
}

func helpFunc() cli.HelpFunc {
	// This should be updated for any commands we want to hide for any reason.
	// Hidden commands can still be executed if you know the command, but
	// aren't shown in any help output. We use this for prerelease functionality
	// or advanced features.
	hidden := map[string]struct{}{
		"inject-connect": {},
	}

	var include []string
	for k := range Commands {
		if _, ok := hidden[k]; !ok {
			include = append(include, k)
		}
	}

	return cli.FilteredHelpFunc(include, cli.BasicHelpFunc("consul-ns1"))
}