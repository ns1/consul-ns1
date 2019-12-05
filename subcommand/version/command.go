package version

import (
	"fmt"

	"github.com/mitchellh/cli"
)

// Command contains objects relating to the subcommand
type Command struct {
	UI      cli.Ui
	Version string
}

// Run executes the subcommand
func (c *Command) Run(_ []string) int {
	c.UI.Output(fmt.Sprintf("consul-ns1 %s", c.Version))
	return 0
}

// Synopsis returns a short description of the subcommand
func (c *Command) Synopsis() string {
	return "Prints the version"
}

// Help returns the help string for the subcommand
func (c *Command) Help() string {
	return ""
}
