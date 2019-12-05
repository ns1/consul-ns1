package catalog

import (
	"fmt"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
)

const (
	// WaitTime is the max time (in seconds) to wait before polling Consul for updates
	WaitTime = 10
)

type consul struct {
	client    *consulapi.Client
	log       hclog.Logger
	ns1Prefix string
	services  map[string]service
	trigger   chan bool
	lock      sync.RWMutex
	stale     bool
	dnsTTL    int64
}

func (c *consul) sync(ns1 *ns1, stop, stopped chan struct{}) {
	defer close(stopped)
	cTriggered := false
	nTriggered := false
	for {
		select {
		case <-c.trigger:
			cTriggered = true
		case <-ns1.trigger:
			nTriggered = true
		case <-stop:
			return
		}

		if cTriggered && nTriggered {
			ns1.log.Debug("Services before upsert", "consul", c.getServices(), "ns1", ns1.getServices())
			upsert := onlyInFirst(c.getServices(), ns1.getServices())
			count := ns1.create(upsert)
			if count > 0 {
				ns1.log.Info("upserted", "count", fmt.Sprintf("%d", count))
			}

			remove := serviceOnlyInFirst(ns1.getServices(), c.getServices())
			count = ns1.remove(remove)
			if count > 0 {
				ns1.log.Info("removed", "count", fmt.Sprintf("%d", count))
			}
			cTriggered = false
			nTriggered = false
		}
	}
}

// getServices returns a copy of currently registered services.  This is a blocking operation.
func (c *consul) getServices() map[string]service {
	c.lock.RLock()
	copy := c.services
	c.lock.RUnlock()
	return copy
}

// setServices replaces the current list of registered services. This is a blocking operation.
func (c *consul) setServices(services map[string]service) {
	c.lock.Lock()
	c.services = services
	c.lock.Unlock()
}

// fetchNodes retrieves the list of Consul nodes
func (c *consul) fetchNodes(service string) ([]*consulapi.CatalogService, error) {
	opts := &consulapi.QueryOptions{AllowStale: c.stale}
	nodes, _, err := c.client.Catalog().Service(service, "", opts)
	if err != nil {
		return nil, fmt.Errorf("error querying services, will retry: %s", err)
	}
	return nodes, err
}

// fetchHealth retrieves the status of health checks associated with a service
func (c *consul) fetchHealth(name string) (consulapi.HealthChecks, error) {
	opts := &consulapi.QueryOptions{AllowStale: c.stale}
	status, _, err := c.client.Health().Checks(name, opts)
	if err != nil {
		return nil, fmt.Errorf("error querying health, will retry: %s", err)
	}
	return status, nil
}

// fetchServices retrieves all known services once the next index after `waitIndex` is reached
// or `WaitTime` has passed.
func (c *consul) fetchServices(waitIndex uint64) (map[string][]string, uint64, error) {
	opts := &consulapi.QueryOptions{
		AllowStale: c.stale,
		WaitIndex:  waitIndex,
		WaitTime:   WaitTime * time.Second,
	}
	services, meta, err := c.client.Catalog().Services(opts)
	if err != nil {
		return services, 0, err
	}
	return services, meta.LastIndex, nil
}

// fetch queries all known services and updates the local `services` cache
func (c *consul) fetch(waitIndex uint64) (uint64, error) {
	cservices, waitIndex, err := c.fetchServices(waitIndex)
	if err != nil {
		return waitIndex, fmt.Errorf("error fetching services: %s", err)
	}
	c.log.Debug(fmt.Sprintf("Services fetched at index %d: %#v", waitIndex, cservices))
	services := c.transformServices(cservices)
	for id, s := range c.transformServices(cservices) {
		// fetch nodes and health for the service and transform
		if cnodes, err := c.fetchNodes(id); err == nil {
			s.nodes = c.transformNodes(cnodes)
		} else {
			c.log.Error("error fetching nodes", "error", err)
			continue
		}
		if chealths, err := c.fetchHealth(id); err == nil {
			s.healths = c.transformHealth(chealths)
		} else {
			c.log.Error("error fetch health", "error", err)
		}
		// set default TTLs
		s.ttls.aRecTTL, s.ttls.srvRecTTL = c.dnsTTL, c.dnsTTL
		services[id] = s
	}
	c.setServices(services)
	return waitIndex, nil
}

// transformHealth transforms Consul `HealthChecks` status into a `service` `healths` enum
func (c *consul) transformHealth(chealths consulapi.HealthChecks) map[string]health {
	healths := map[string]health{}
	for _, h := range chealths {
		switch h.Status {
		case "passing":
			healths[h.ServiceID] = passing
		case "critical":
			healths[h.ServiceID] = critical
		default:
			healths[h.ServiceID] = unknown
		}
	}
	return healths
}

// transformNodes transforms a list of Consul nodes for a service into a map of nodes and answers
func (c *consul) transformNodes(cnodes []*consulapi.CatalogService) map[string]node {
	nodes := map[string]node{}
	for _, n := range cnodes {
		address := n.ServiceAddress
		if len(address) == 0 {
			address = n.Address
		}
		if _, ok := nodes[address]; !ok {
			nodes[address] = node{}
		}
		node := nodes[address]
		if node.aRecAnswer == "" {
			node.aRecAnswer = address
		}
		if node.srvRecAnswers == nil {
			node.srvRecAnswers = map[int]srvAnswer{}
		}
		if _, ok := node.srvRecAnswers[n.ServicePort]; !ok {
			node.srvRecAnswers[n.ServicePort] = srvAnswer{
				priority: 1,
				weight:   1,
				port:     int64(n.ServicePort),
				address:  address,
			}
		}
		nodes[address] = node
	}
	return nodes

}

// transformServices transforms a map of services to the format required by local cache
func (c *consul) transformServices(cservices map[string][]string) map[string]service {
	services := make(map[string]service, len(cservices))
	for k := range cservices {
		s := service{id: k, name: k, consulID: k}
		services[s.name] = s
	}
	return services
}

// fetchIndefinitely is the main event loop for fetching services and handling channel events
func (c *consul) fetchIndefinitely(stop, stopped chan struct{}) {
	defer close(stopped)
	waitIndex := uint64(1)
	subsequentErrors := 0
	for {
		c.log.Debug(fmt.Sprintf("Fetching services at index %d", waitIndex))
		newIndex, err := c.fetch(waitIndex)
		if err != nil {
			c.log.Error("error fetching", "error", err.Error())
			subsequentErrors++
			if subsequentErrors > 10 {
				return
			}
			time.Sleep(500 * time.Millisecond)
		} else {
			subsequentErrors = 0
			waitIndex = newIndex
			c.trigger <- true
		}
		select {
		case <-stop:
			return
		default:
		}
	}
}
