package synccatalog

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
	"github.com/nsone/consul-ns1/catalog"
	"github.com/nsone/consul-ns1/subcommand"
)

// Command is the command for syncing the A
type Command struct {
	UI cli.Ui

	flags                *flag.FlagSet
	http                 *flags.HTTPFlags
	flagNS1ServicePrefix string
	flagNS1PollInterval  string
	flagNS1DNSTTL        int64
	flagNS1Endpoint      string
	flagNS1Domain        string
	flagNS1APIKey        string
	flagNS1IgnoreSSL     bool

	once sync.Once
	help string
}

func (c *Command) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.flagNS1ServicePrefix, "ns1-service-prefix",
		"", "A prefix to prepend to all services written to NS1 from Consul. "+
			"If this is not set then services will have no prefix.")
	c.flags.StringVar(&c.flagNS1PollInterval, "ns1-poll-interval",
		"30s", "The interval between fetching from NS1. "+
			"Accepts a sequence of decimal numbers, each with optional "+
			"fraction and a unit suffix, such as \"300ms\", \"10s\", \"1.5m\". "+
			"(Defaults to 30s)")
	c.flags.Int64Var(&c.flagNS1DNSTTL, "ns1-dns-ttl",
		60, "DNS TTL for services created in NS1 in seconds. (Defaults to 60)")
	c.flags.StringVar(&c.flagNS1Endpoint, "ns1-endpoint", "",
		"The absolute URL of the NS1 API endpoint. (Defaults to https://api.nsone.net/v1/)")
	c.flags.StringVar(&c.flagNS1Domain, "ns1-domain", "",
		"Name of the DNS domain in NS1 to create records for Consul services in. "+
			"WARNING: consul-ns1 will delete any records in this zone that do not correspond to a Consul service.")
	c.flags.StringVar(&c.flagNS1APIKey, "ns1-apikey", "",
		"The API key to use when communicating with NS1.  This can also be specified via the "+
			"NS1_APIKEY environment variable.")
	c.flags.BoolVar(&c.flagNS1IgnoreSSL, "ns1-ignoressl", false,
		"Ignore SSL validation when communicating with NS1. (Defaults to false)")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

// Run initializes API clients and the main program loop
func (c *Command) Run(args []string) int {
	c.once.Do(c.init)
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	if len(c.flags.Args()) > 0 {
		c.UI.Error("Should have no non-flag arguments.")
		return 1
	}
	if c.flagNS1Domain == "" {
		c.UI.Error("Please provide -ns1-domain")
		return 1
	}
	ns1Client, err := subcommand.NS1Client(c.flagNS1Endpoint, c.flagNS1APIKey, c.flagNS1IgnoreSSL)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error retrieving NS1 client: %s", err))
		return 1
	}

	consulClient, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})
	go catalog.Sync(
		c.flagNS1ServicePrefix, c.flagNS1PollInterval, c.flagNS1DNSTTL,
		c.flagNS1Domain, c.getStaleWithDefaultTrue(), ns1Client, consulClient,
		stop, stopped,
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	select {
	// Unexpected failure
	case <-stopped:
		return 1
	case <-sigCh:
		c.UI.Info("shutting down...")
		close(stop)
		<-stopped
	}
	return 0
}

func (c *Command) getStaleWithDefaultTrue() bool {
	stale := true
	c.flags.Visit(func(f *flag.Flag) {
		if f.Name == "stale" {
			stale = c.http.Stale()
			return
		}
	})
	return stale
}

// Synopsis returns a short description of the program
func (c *Command) Synopsis() string { return synopsis }

// Help returns usage info for the program
func (c *Command) Help() string {
	c.once.Do(c.init)
	return c.help
}

const synopsis = "Sync NS1 and Consul services."
const help = `
Usage: consul-ns1 sync-catalog [options]

  Sync NS1 records with the Consul service catalog.
  This enables consumers of NS1 DNS to discover and communicate with external
  services.

`
