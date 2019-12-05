package catalog

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

type zone struct {
	id   string
	name string
}

type zoneService interface {
	Get(z string) (*dns.Zone, *http.Response, error)
}
type recordService interface {
	Create(r *dns.Record) (*http.Response, error)
	Update(r *dns.Record) (*http.Response, error)
	Delete(zone, domain, t string) (*http.Response, error)
	Get(zone, domain, t string) (*dns.Record, *http.Response, error)
}

// ns1APIClient wraps the NS1 SDK for mocking
type ns1APIClient struct {
	Zones   zoneService
	Records recordService
}

type ns1 struct {
	client       *ns1APIClient
	log          hclog.Logger
	serviceZone  zone
	ns1Prefix    string
	services     map[string]service
	trigger      chan bool
	lock         sync.RWMutex
	pollInterval time.Duration
	dnsTTL       int64
}

// setupServiceZone attempts to fetch a zone and store it's metadata to use when sync'ing services
func (n *ns1) setupServiceZone(zoneName string) error {
	zone, err := n.fetchZone(zoneName)
	if err != nil {
		return err
	}
	n.serviceZone = n.transformZone(zone)
	return nil
}

// getServices returns a copy of currently registered services.  This is a blocking operation.
func (n *ns1) getServices() map[string]service {
	n.lock.RLock()
	copy := n.services
	n.lock.RUnlock()
	return copy
}

// setServices replaces the current list of registered services. This is a blocking operation.
func (n *ns1) setServices(services map[string]service) {
	n.lock.Lock()
	n.services = services
	n.lock.Unlock()
}

// fetch queries records from the service zone and updates the local `services` cache
func (n *ns1) fetch() error {
	n.log.Debug("Performing fetch from NS1", "zone", n.serviceZone.name)
	zone, err := n.fetchZone(n.serviceZone.name)
	if err != nil {
		return err
	}
	services := n.transformZoneRecords(zone)
	n.setServices(services)
	return nil
}

// fetchZone retrieves a zone from NS1
func (n *ns1) fetchZone(zoneName string) (*dns.Zone, error) {
	ns1Zone, _, err := n.client.Zones.Get(zoneName)
	if err != nil {
		return nil, err
	}
	return ns1Zone, nil
}

// transformZone transforms a NS1 zone into a zone required by local cache
func (n *ns1) transformZone(ns1Zone *dns.Zone) zone {
	return zone{id: ns1Zone.ID, name: ns1Zone.Zone}
}

// transformZoneRecords transforms records in a NS1 zone into a map of services
func (n *ns1) transformZoneRecords(ns1Zone *dns.Zone) map[string]service {
	services := map[string]service{}
	for _, record := range ns1Zone.Records {
		if record.Type != "A" && record.Type != "SRV" {
			n.log.Debug("Non-service record type found in zone, ignoring", "ID", fmt.Sprintf("%s", record.ID))
			continue
		}
		// Trim zone name and prefix, if applicable
		serviceName := strings.TrimPrefix(record.Domain, n.ns1Prefix)
		serviceName = strings.TrimSuffix(serviceName, "."+n.serviceZone.name)

		// Service could already exist, since multiple records map to a single service
		var svc service
		if s, ok := services[serviceName]; !ok {
			svc = service{name: serviceName}
		} else {
			svc = s
		}
		// Populate ns1IDs and TTLs
		if record.Type == "A" {
			svc.ns1IDs.aRecID = record.ID
			svc.ttls.aRecTTL = int64(record.TTL)
		} else if record.Type == "SRV" {
			svc.ns1IDs.srvRecID = record.ID
			svc.ttls.srvRecTTL = int64(record.TTL)
		}
		// Populate node
		if len(record.ShortAns) > 0 && svc.nodes == nil {
			svc.nodes = map[string]node{}
		}
		for _, ans := range record.ShortAns {
			var address string
			ansFields := strings.Fields(ans)
			if len(ansFields) == 4 {
				address = ansFields[3]
			} else {
				address = ansFields[0]
			}

			var ansNode node
			if n, ok := svc.nodes[address]; !ok {
				ansNode = node{}
			} else {
				ansNode = n
			}

			if record.Type == "A" {
				ansNode.aRecAnswer = address
			} else if record.Type == "SRV" && len(ansFields) == 4 {
				if ansNode.srvRecAnswers == nil {
					ansNode.srvRecAnswers = map[int]srvAnswer{}
				}
				priority, err := strconv.ParseInt(ansFields[0], 10, 64)
				if err != nil {
					n.log.Error("Unable to parse priority in SRV answer", ans)
					continue
				}
				weight, err := strconv.ParseInt(ansFields[1], 10, 64)
				if err != nil {
					n.log.Error("Unable to parse weight in SRV answer", ans)
					continue
				}
				port, err := strconv.ParseInt(ansFields[2], 10, 64)
				if err != nil {
					n.log.Error("Unable to parse port in SRV answer", ans)
					continue
				}
				ansNode.srvRecAnswers[int(port)] = srvAnswer{
					priority: priority,
					weight:   weight,
					port:     port,
					address:  address,
				}
			}
			svc.nodes[address] = ansNode
		}

		services[serviceName] = svc
	}
	return services
}

// upsertRecord creates a DNS record, if no ID is given.
// If an ID is given, it updates an existing record.
func (n *ns1) upsertRecord(id string, rec *dns.Record) error {
	var err error
	if id == "" {
		n.log.Debug("Creating record", "domain", rec.Domain, "type", rec.Type, "Answers", rec.Answers)
		_, err = n.client.Records.Create(rec)
	} else {
		n.log.Debug("Updating record", "domain", rec.Domain, "type", rec.Type, "Answers", rec.Answers, "Filters", rec.Filters)
		_, err = n.client.Records.Update(rec)
	}
	if err != nil {
		return err
	}

	return nil
}

// generateRecord creates a new dns.Record struct for a service of type t.
// If no id is given a new struct with default values is returned.
// If an id is given, record values are fetched from NS1. Existing answers will be removed and TTL will be overwritten.
func (n *ns1) generateRecord(id, name, t string) (*dns.Record, error) {
	var err error
	domain := name + "." + n.serviceZone.name
	rec := &dns.Record{}
	if id == "" {
		rec = dns.NewRecord(n.serviceZone.name, domain, t)
	} else {
		rec, _, err = n.client.Records.Get(n.serviceZone.name, domain, t)
		if err != nil {
			return nil, err
		}
	}
	rec.Answers = []*dns.Answer{}
	rec.TTL = int(n.dnsTTL)
	return rec, nil
}

// Create creates or updates records in NS1 for a set of services. Returns the number of created or updated records.
func (n *ns1) create(services map[string]service) int32 {
	wg := sync.WaitGroup{}
	var count int32
	for k, s := range services {
		name := n.ns1Prefix + k
		aRec, err := n.generateRecord(s.ns1IDs.aRecID, name, "A")
		if err != nil {
			n.log.Error("cannot fetch A record for service, generating new record", "name", name, "id", s.ns1IDs.aRecID, "error", err.Error())
			aRec, _ = n.generateRecord("", name, "A")
		}
		srvRec, err := n.generateRecord(s.ns1IDs.srvRecID, name, "SRV")
		if err != nil {
			n.log.Error("cannot fetch SRV record for service, generating new record", "name", name, "domain", s.ns1IDs.srvRecID, "error", err.Error())
			srvRec, _ = n.generateRecord("", name, "SRV")
		}

		// Add answers
		for _, node := range s.nodes {
			if node.aRecAnswer != "" {
				aRec.AddAnswer(dns.NewAv4Answer(node.aRecAnswer))
			}

			for _, a := range node.srvRecAnswers {
				srvFields := strings.Fields(a.String())
				srvRec.AddAnswer(dns.NewAnswer(srvFields))
			}
		}

		// Update records in NS1
		wg.Add(2)
		go n.upsertRecordWorker(&wg, s.ns1IDs.aRecID, aRec, &count)
		go n.upsertRecordWorker(&wg, s.ns1IDs.srvRecID, srvRec, &count)
	}
	wg.Wait()
	return count
}

// upsertRecordWorker wraps upsertRecord for coordination via WaitGroup and mutates count if upsertion was succesful
func (n *ns1) upsertRecordWorker(wg *sync.WaitGroup, recID string, rec *dns.Record, count *int32) {
	err := n.upsertRecord(recID, rec)
	if err != nil {
		n.log.Error("cannot create or update record for service", "domain", rec.Domain, "type", rec.Type, "error", err.Error())
	} else {
		atomic.AddInt32(count, 1)
	}
	wg.Done()
}

// removeRecordWorker wraps ns1.client.Records.Delete for coordination via WaitGroup
// and mutates count if deletion was successful
func (n *ns1) removeRecordWorker(wg *sync.WaitGroup, zone, domain, recType string, count *int32) {
	n.log.Debug("Removing record", "zone", n.serviceZone.name, "domain", domain, "type", recType)
	_, err := n.client.Records.Delete(zone, domain, recType)
	if err != nil {
		n.log.Error("Record for service could not be deleted", "zone", zone, "domain", domain, "type", recType, "error", err.Error())
	} else {
		atomic.AddInt32(count, 1)
	}
	wg.Done()
}

// Remove deletes a record for a service from NS1, it ignores service nodes
// as nodes are sync'ed with answers in Create
func (n *ns1) remove(services map[string]service) int32 {
	wg := sync.WaitGroup{}
	var count int32
	for k, s := range services {
		domain := ""
		if k == n.serviceZone.name {
			// handle apex record
			domain = n.serviceZone.name
		} else {
			name := n.ns1Prefix + k
			domain = name + "." + n.serviceZone.name
		}
		if len(s.ns1IDs.aRecID) != 0 {
			wg.Add(1)
			go n.removeRecordWorker(&wg, n.serviceZone.name, domain, "A", &count)
		}
		if len(s.ns1IDs.srvRecID) != 0 {
			wg.Add(1)
			go n.removeRecordWorker(&wg, n.serviceZone.name, domain, "SRV", &count)
		}
	}
	wg.Wait()
	return count
}

func (n *ns1) fetchIndefinitely(stop, stopped chan struct{}) {
	defer close(stopped)
	for {
		err := n.fetch()
		if err != nil {
			n.log.Error("error fetching", "error", err.Error())
		} else {
			n.trigger <- true
		}
		select {
		case <-stop:
			return
		case <-time.After(n.pollInterval):
			continue
		}
	}
}
