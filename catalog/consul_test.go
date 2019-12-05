package catalog

import (
	"testing"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestConsulTransformServices(t *testing.T) {
	c := consul{}
	services := map[string][]string{"s1": {"abc"}}
	expected := map[string]service{"s1": {id: "s1", name: "s1", consulID: "s1"}}

	require.Equal(t, expected, c.transformServices(services))
}

func TestConsulTransformNodes(t *testing.T) {
	c := consul{}
	nodes := []*consulapi.CatalogService{
		{
			Address:     "1.1.1.1",
			ServicePort: 3,
			ServiceID:   "s1",
			ServiceMeta: map[string]string{"A": "B"},
		},
		{
			Address:     "1.1.1.1",
			ServicePort: 4,
			ServiceID:   "s1",
			ServiceMeta: map[string]string{"A": "B"},
		},
		{
			Address:     "2.2.2.2",
			ServicePort: 3,
			ServiceID:   "s1",
			ServiceMeta: map[string]string{"A": "B"},
		},
	}
	expected := map[string]node{
		"1.1.1.1": {
			aRecAnswer: "1.1.1.1",
			srvRecAnswers: map[int]srvAnswer{
				3: srvAnswer{priority: 1, weight: 1, port: 3, address: "1.1.1.1"},
				4: srvAnswer{priority: 1, weight: 1, port: 4, address: "1.1.1.1"},
			},
		},
		"2.2.2.2": {
			aRecAnswer: "2.2.2.2",
			srvRecAnswers: map[int]srvAnswer{
				3: srvAnswer{priority: 1, weight: 1, port: 3, address: "2.2.2.2"},
			},
		},
	}
	require.Equal(t, expected, c.transformNodes(nodes))
}

func TestConsulTransformHeath(t *testing.T) {
	c := consul{}
	healths := consulapi.HealthChecks{
		&consulapi.HealthCheck{Status: "passing", ServiceID: "s1"},
		&consulapi.HealthCheck{Status: "critical", ServiceID: "s2"},
		&consulapi.HealthCheck{Status: "warning", ServiceID: "s3"},
	}
	expected := map[string]health{
		"s1": passing,
		"s2": critical,
		"s3": unknown,
	}
	require.Equal(t, expected, c.transformHealth(healths))
}
