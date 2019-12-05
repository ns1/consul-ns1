# Consul-NS1

`consul-ns1` syncs the services in a Consul datacenter to a DNS zone in NS1.  For each service in Consul's catalog, a corresponding A record and SRV record will be created within a specified zone.  Additionally, an answer will be added to the DNS records for each instance of the service.  This enables service discovery via DNS.

# Installation

## Binary Release

1. Download a pre-compiled, released version from the the [Consul-NS1 releases page](releases).

1. Extract the binary using `unzip` or `tar`.

1. Move the binary into `$PATH`.

## From Source
If you prefer to build your own binary from the latest release of the source code, make sure you have a correctly configured **Go >= 1.13** environment.

1. Ensure support for Go Modules is enabled:

```shell
export GO111MODULE=on
```

2. Download `consul-ns1`:

```shell
$ go get github.com/ns1/consul-ns1
```
At this point, the binary should be in *$GOPATH/bin*.

3.  If you'd like to rebuild the binary from the project source code:

```shell
$ cd $GOPATH/github.com/ns1/consul-ns1
$ go install
```

# Usage

`consul-ns1` needs to be connected to both a Consul cluster and NS1 (Managed DNS or Private DNS/Enterprise DDI instance), in order to sync Consul services to NS1.

In order to help with connecting to a Consul cluster, `consul-ns1` provides all the flags you might need including the possibility to set an ACL token.

To connect to NS1, you must specify your API key in an environment variable named `NS1_APIKEY`
or via the `-ns1-apikey` flag.  To sync to a DNS zone that is managed by a Private DNS/Enterprise DDI instance of NS1, you can specify the full URL of the NS1 API endpoint via the `-ns1-endpoint` flag.

This is how `consul-ns1` could be invoked to sync Consul services to a zone in Managed DNS:

```shell
$ ./consul-ns1 sync-catalog -ns1-domain=myservices.com
```

# Contributing

Contributions, ideas and criticisms are all welcome.

## Testing

```shell
$ go test
```

# License

Apache2 - see the included [LICENSE](LICENSE.txt) file for more information
