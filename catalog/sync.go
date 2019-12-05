package catalog

import (
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	ns1api "gopkg.in/ns1/ns1-go.v2/rest"
)

// Sync consul->ns1
func Sync(ns1Prefix, ns1PollInterval string, ns1DNSTTL int64, ns1Domain string, stale bool, ns1Client *ns1api.Client, consulClient *consulapi.Client, stop, stopped chan struct{}) {
	defer close(stopped)
	log := hclog.Default().Named("sync")
	consul := consul{
		client:    consulClient,
		log:       hclog.Default().Named("consul"),
		trigger:   make(chan bool, 1),
		ns1Prefix: ns1Prefix,
		stale:     stale,
		dnsTTL:    ns1DNSTTL,
	}
	pollInterval, err := time.ParseDuration(ns1PollInterval)
	if err != nil {
		log.Error("cannot parse ns1 pull interval", "error", err)
		return
	}
	ns1 := ns1{
		client:       &ns1APIClient{Zones: ns1Client.Zones, Records: ns1Client.Records},
		log:          hclog.Default().Named("ns1"),
		ns1Prefix:    ns1Prefix,
		trigger:      make(chan bool, 1),
		pollInterval: pollInterval,
		dnsTTL:       ns1DNSTTL,
	}
	/*ns1.client = &ns1APIClient{
		Zones:   ns1Client.Zones,
		Records: ns1Client.Records,
	}*/
	err = ns1.setupServiceZone(ns1Domain)
	if err != nil {
		switch err {
		case ns1api.ErrZoneMissing:
			log.Error(fmt.Sprintf("zone %s not found in NS1", ns1Domain), "error", err)
		default:
			log.Error(fmt.Sprintf("cannot sync to domain %s", ns1Domain), "error", err)
		}
		return
	}

	fetchConsulStop := make(chan struct{})
	fetchConsulStopped := make(chan struct{})
	go consul.fetchIndefinitely(fetchConsulStop, fetchConsulStopped)
	fetchNS1Stop := make(chan struct{})
	fetchNS1Stopped := make(chan struct{})
	go ns1.fetchIndefinitely(fetchNS1Stop, fetchNS1Stopped)

	toNS1Stop := make(chan struct{})
	toNS1Stopped := make(chan struct{})

	go consul.sync(&ns1, toNS1Stop, toNS1Stopped)

	select {
	case <-stop:
		close(toNS1Stop)
		close(fetchNS1Stop)
		close(fetchConsulStop)
		<-fetchConsulStopped
		<-fetchNS1Stopped
		<-toNS1Stopped
	case <-fetchNS1Stopped:
		log.Info("problem with NS1 fetch. shutting down...")
		close(toNS1Stop)
		close(fetchConsulStop)
		<-toNS1Stopped
		<-fetchConsulStopped
	case <-fetchConsulStopped:
		log.Info("problem with consul fetch. shutting down...")
		close(toNS1Stop)
		close(fetchNS1Stop)
		<-toNS1Stopped
		<-fetchNS1Stopped
	case <-toNS1Stopped:
		log.Info("problem with NS1 sync. shutting down...")
		close(fetchConsulStop)
		close(fetchNS1Stop)
		<-fetchConsulStopped
		<-fetchNS1Stopped
	}
}
