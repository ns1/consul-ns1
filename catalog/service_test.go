package catalog

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodesAreEqual(t *testing.T) {
	type variant struct {
		a        map[string]node
		b        map[string]node
		expected bool
	}

	table := map[string]variant{
		"Empty nodes": {
			a:        map[string]node{},
			b:        map[string]node{},
			expected: true,
		},
		"Node only in first": {
			a:        map[string]node{"h1": {}},
			b:        map[string]node{},
			expected: false,
		},
		"Node only in second": {
			a:        map[string]node{},
			b:        map[string]node{"h1": {}},
			expected: false,
		},
		"Node in both": {
			a:        map[string]node{"h1": {}},
			b:        map[string]node{"h1": {}},
			expected: true,
		},
		"Extra node in second": {
			a:        map[string]node{"h1": {}},
			b:        map[string]node{"h1": {}, "h2": {}},
			expected: false,
		},
		"A record answer only in first": {
			a:        map[string]node{"h1": {aRecAnswer: "1.1.1.1"}},
			b:        map[string]node{"h1": {}},
			expected: false,
		},
		"A record answer only in second": {
			a:        map[string]node{"h1": {}},
			b:        map[string]node{"h1": {aRecAnswer: "1.1.1.1"}},
			expected: false,
		},
		"A record answer in both": {
			a:        map[string]node{"h1": {aRecAnswer: "1.1.1.1"}},
			b:        map[string]node{"h1": {aRecAnswer: "1.1.1.1"}},
			expected: true,
		},
		"SRV record answer only in first": {
			a: map[string]node{
				"h1": {
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
			b:        map[string]node{"h1": {}},
			expected: false,
		},
		"SRV record answer only in second": {
			a: map[string]node{"h1": {}},
			b: map[string]node{
				"h1": {
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
			expected: false,
		},
		"Extra SRV record answer in second": {
			a: map[string]node{
				"h1": {
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
			b: map[string]node{
				"h1": {
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
						2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
					},
				},
			},
			expected: false,
		},
		"SRV record answer in both": {
			a: map[string]node{
				"h1": {
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
			b: map[string]node{
				"h1": {
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
			expected: true,
		},
	}

	for name, v := range table {
		assert.Equal(t, v.expected, nodesAreEqual(v.a, v.b), fmt.Sprintf("Test case: %s", name))
	}
}

func TestOnlyInFirst(t *testing.T) {
	type variant struct {
		a        map[string]service
		b        map[string]service
		expected map[string]service
	}

	table := map[string]variant{
		"Empty services": {
			a:        map[string]service{},
			b:        map[string]service{},
			expected: map[string]service{},
		},
		"Service only in first": {
			a:        map[string]service{"s1": {}},
			b:        map[string]service{},
			expected: map[string]service{"s1": {}},
		},
		"Service only in both": {
			a:        map[string]service{"s2": {}},
			b:        map[string]service{"s2": {}},
			expected: map[string]service{},
		},
		"Extra service in first": {
			a:        map[string]service{"s3": {}, "s4": {}},
			b:        map[string]service{"s3": {}},
			expected: map[string]service{"s4": {}},
		},
		"Node only in first": {
			a: map[string]service{
				"s5": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
			b: map[string]service{
				"s5": {nodes: map[string]node{"h2": {aRecAnswer: "2.2.2.2"}}},
			},
			expected: map[string]service{
				"s5": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
		},
		"Extra node in second": {
			a: map[string]service{
				"s5": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
			b: map[string]service{
				"s5": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}, "h2": {aRecAnswer: "2.2.2.2"}}},
			},
			expected: map[string]service{
				"s5": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
		},
		"SRV answer only in first": {
			a: map[string]service{
				"s6": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
							},
						},
					},
				},
			},
			b: map[string]service{
				"s6": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
							},
						},
					},
				},
			},
			expected: map[string]service{
				"s6": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
							},
						},
					},
				},
			},
		},
		"Extra SRV answer in second": {
			a: map[string]service{
				"s7": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
							},
						},
					},
				},
			},
			b: map[string]service{
				"s7": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
								2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
							},
						},
					},
				},
			},
			expected: map[string]service{
				"s7": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
							},
						},
					},
				},
			},
		},
		"Extra SRV answer only in first": {
			a: map[string]service{
				"s8": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
								2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
							},
						},
					},
				},
			},
			b: map[string]service{
				"s8": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
							},
						},
					},
				},
			},
			expected: map[string]service{
				"s8": {
					nodes: map[string]node{
						"h1": {
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
								2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
							},
						},
					},
				},
			},
		},
		"Service ID and Name only in first": {
			a: map[string]service{
				"s9": {
					id:     "id",
					name:   "name",
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h1": {}, "h2": {}},
				},
			},
			b: map[string]service{
				"s9": {
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h2": {}},
				},
			},
			expected: map[string]service{
				"s9": {
					id:     "id",
					name:   "name",
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h1": {}, "h2": {}},
				},
			},
		},
		"Service ID and Name only in second": {
			a: map[string]service{
				"s10": {
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h1": {}, "h2": {}},
				},
			},
			b: map[string]service{
				"s10": {
					id:     "id",
					name:   "name",
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h2": {}},
				},
			},
			expected: map[string]service{
				"s10": {
					id:     "id",
					name:   "name",
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h1": {}, "h2": {}},
				},
			},
		},
		"Record IDs only in first": {
			a: map[string]service{
				"s11": {
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h1": {}, "h2": {}},
				},
			},
			b: map[string]service{
				"s11": {
					nodes: map[string]node{"h2": {}},
				},
			},
			expected: map[string]service{
				"s11": {
					ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"},
					nodes:  map[string]node{"h1": {}, "h2": {}},
				},
			},
		},
		"TTLs don't match": {
			a:        map[string]service{"s12": {ttls: recordTTLs{aRecTTL: 1, srvRecTTL: 2}}},
			b:        map[string]service{"s12": {ttls: recordTTLs{aRecTTL: 3, srvRecTTL: 4}}},
			expected: map[string]service{"s12": {ttls: recordTTLs{aRecTTL: 1, srvRecTTL: 2}}},
		},
	}
	for name, v := range table {
		assert.Equal(t, v.expected, onlyInFirst(v.a, v.b), fmt.Sprintf("Test case: %s", name))
	}
}
