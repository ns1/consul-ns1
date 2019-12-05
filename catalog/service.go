package catalog

import "fmt"

type health string

const (
	passing  health = "passing"
	critical health = "critical"
	unknown  health = ""
)

type service struct {
	id       string
	name     string
	nodes    map[string]node
	healths  map[string]health
	ttls     recordTTLs
	ns1IDs   recordIDs
	consulID string
}

type node struct {
	host          string
	datacenter    string
	consulID      string
	aRecAnswer    string
	srvRecAnswers map[int]srvAnswer
}

type recordIDs struct {
	aRecID   string
	srvRecID string
}

type recordTTLs struct {
	aRecTTL   int64
	srvRecTTL int64
}

type srvAnswer struct {
	priority int64
	weight   int64
	port     int64
	address  string
}

func (a srvAnswer) String() string {
	return fmt.Sprintf("%d %d %d %s", a.priority, a.weight, a.port, a.address)
}

// serviceOnlyInFirst compares two maps of services and returns a map of the ones that only exist in the first map.
// It ignores diffs between nodes or answers and only includes answer in result if serviceA does not exist servicesB.
func serviceOnlyInFirst(servicesA, servicesB map[string]service) map[string]service {
	result := map[string]service{}
	for k, sa := range servicesA {
		if _, ok := servicesB[k]; !ok {
			result[k] = sa
		}
	}
	return result
}

// nodesAreEqual determines if two maps of nodes are considered equal
func nodesAreEqual(expected, actual map[string]node) bool {
	if len(expected) != len(actual) {
		return false
	}
	for h, expectedNode := range expected {
		if _, ok := actual[h]; !ok {
			return false
		}
		actualNode := actual[h]
		// compare A record answers
		if actualNode.aRecAnswer != expectedNode.aRecAnswer {
			return false
		}
		// compare SRV record answers
		if len(expectedNode.srvRecAnswers) != len(actualNode.srvRecAnswers) {
			return false
		}
		for p, expectedSrv := range expectedNode.srvRecAnswers {
			if actualSrv, ok := actualNode.srvRecAnswers[p]; !ok || expectedSrv != actualSrv {
				return false
			}
		}
	}

	return true
}

// onlyInFirst compares two maps of services and returns a map of the ones that only exist in the first map.
// On any diff between a service's nodes in A vs B, all nodes are included return map.
func onlyInFirst(servicesA, servicesB map[string]service) map[string]service {
	result := map[string]service{}
	for k, sa := range servicesA {
		if sb, ok := servicesB[k]; !ok {
			// service k is not defined in servicesB, should be in results
			result[k] = sa
		} else {
			nodes := map[string]node{}
			// if nodes aren't equal or TTLs don't match
			if !nodesAreEqual(sa.nodes, sb.nodes) || sa.ttls != sb.ttls {
				nodes = sa.nodes
				id := sa.id
				if len(sa.id) == 0 {
					id = sb.id
				}
				name := sa.name
				if len(sa.name) == 0 {
					name = sb.name
				}
				ns1IDs := recordIDs{
					aRecID:   sa.ns1IDs.aRecID,
					srvRecID: sa.ns1IDs.srvRecID,
				}
				if len(ns1IDs.aRecID) == 0 {
					ns1IDs.aRecID = sb.ns1IDs.aRecID
				}
				if len(ns1IDs.srvRecID) == 0 {
					ns1IDs.srvRecID = sb.ns1IDs.srvRecID
				}
				ttls := recordTTLs{
					aRecTTL:   sa.ttls.aRecTTL,
					srvRecTTL: sa.ttls.srvRecTTL,
				}
				if ttls.aRecTTL == 0 {
					ttls.aRecTTL = sb.ttls.aRecTTL
				}
				if ttls.srvRecTTL == 0 {
					ttls.srvRecTTL = sb.ttls.srvRecTTL
				}
				s := service{
					id:     id,
					name:   name,
					ttls:   ttls,
					ns1IDs: ns1IDs,
				}
				if len(nodes) > 0 {
					s.nodes = nodes
				}
				result[k] = s
			}
		}
	}
	return result
}
