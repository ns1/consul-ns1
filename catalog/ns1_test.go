package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"gopkg.in/ns1/ns1-go.v2/rest/model/filter"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-hclog"
	ns1api "gopkg.in/ns1/ns1-go.v2/rest"
	"gopkg.in/ns1/ns1-go.v2/rest/model/data"
	"gopkg.in/ns1/ns1-go.v2/rest/model/dns"
)

// mockZoneService fulfils the zoneService interface for mocking ns1-go
type mockZoneService struct{}

func (s *mockZoneService) Get(z string) (*dns.Zone, *http.Response, error) {
	if z == "test.zone" {
		zone := &dns.Zone{
			ID:   "57d95da659272400013334de",
			Zone: "test.zone",
			Records: []*dns.ZoneRecord{
				{
					Domain:   "s1.test.zone",
					ID:       "57d95da659272400013334dd",
					ShortAns: []string{"1.1.1.1"},
					Type:     "A",
				},
				{
					Domain:   "s1.test.zone",
					ID:       "57d95da659272400013334dc",
					ShortAns: []string{"1 1 1 1.1.1.1"},
					Type:     "SRV",
				},
				{
					Domain:   "s2.test.zone",
					ID:       "57d95da659272400013334db",
					ShortAns: []string{"1 1 2 2.2.2.2"},
					Type:     "SRV",
				},
			},
		}
		return zone, nil, nil
	}
	return nil, nil, errors.New("Expected z=test.zone")
}

// expectCreateRecordService fulfils the recordService interface for mocking calls to
// to ns1-go RecordService to create a record
type expectCreateRecordService struct {
	callCount int
	records   []*dns.Record
}

func (s *expectCreateRecordService) Create(r *dns.Record) (*http.Response, error) {
	s.callCount++
	s.records = append(s.records, r)
	return nil, nil
}

func (s *expectCreateRecordService) Update(r *dns.Record) (*http.Response, error) {
	return nil, errors.New("Expected Record Create, got Update")
}

func (s *expectCreateRecordService) Delete(zone, domain, t string) (*http.Response, error) {
	s.callCount++
	return nil, nil
}

func (s *expectCreateRecordService) Get(zone, domain, t string) (*dns.Record, *http.Response, error) {
	return nil, nil, errors.New("Expected Record Create, got Get")
}

// expectUpdateRecordService fulfils the recordService interface for mocking calls to
// to ns1-go RecordService to update a record
type expectUpdateRecordService struct {
	callCount int
}

func (s *expectUpdateRecordService) Create(r *dns.Record) (*http.Response, error) {
	return nil, errors.New("Expected Record Update, got Create")
}

func (s *expectUpdateRecordService) Update(r *dns.Record) (*http.Response, error) {
	s.callCount++
	return nil, nil
}

func (s *expectUpdateRecordService) Delete(zone string, domain string, t string) (*http.Response, error) {
	s.callCount++
	return nil, nil
}

func (s *expectUpdateRecordService) Get(zone, domain, t string) (*dns.Record, *http.Response, error) {
	return nil, nil, errors.New("Expected Record Update, got Get")
}

// expectGetRecordService fulfils the recordService interface for mocking calls to
// to ns1-go RecordService to update a record
type expectGetRecordService struct {
	callCount int
}

func (s *expectGetRecordService) Create(r *dns.Record) (*http.Response, error) {
	return nil, errors.New("Expected Record Get, got Create")
}

func (s *expectGetRecordService) Update(r *dns.Record) (*http.Response, error) {
	return nil, errors.New("Expected Record Get, got Update")
}

func (s *expectGetRecordService) Delete(zone string, domain string, t string) (*http.Response, error) {
	return nil, errors.New("Expected Record Get, got Delete")
}

func (s *expectGetRecordService) Get(zone, domain, t string) (*dns.Record, *http.Response, error) {
	s.callCount++

	if zone == "test.zone" && domain == "s1.test.zone" && t == "A" {
		return &dns.Record{
			Zone:   "test.zone",
			Domain: "s1.test.zone",
			Type:   "A",
			Answers: []*dns.Answer{
				{Rdata: []string{"1.1.1.1"}},
			},
			Filters: []*filter.Filter{
				{Type: "shuffle", Config: filter.Config{}},
			},
		}, nil, nil
	}
	return nil, nil, fmt.Errorf("Expected zone=test.zone domain=s1.test.zone t=A"+
		"got zone=%s domain=%s t=%s", zone, domain, t)
}

// expectDeleteRecordService fulfils the recordService interface for mocking calls to
// to ns1-go RecordService to delete a record
type expectDeleteRecordService struct {
	callCount int
	records   []*dns.Record
	mux       *sync.Mutex
}

func (s *expectDeleteRecordService) Create(r *dns.Record) (*http.Response, error) {
	return nil, errors.New("Expected Record Delete, got Create")
}

func (s *expectDeleteRecordService) Update(r *dns.Record) (*http.Response, error) {
	return nil, errors.New("Expected Record Delete, got Update")
}

func (s *expectDeleteRecordService) Delete(zone string, domain string, t string) (*http.Response, error) {
	s.mux.Lock()
	s.callCount++
	for i, r := range s.records {
		if r.Zone == zone && r.Domain == domain && r.Type == t {
			s.records = append(s.records[:i], s.records[i+1:]...)
		}
	}
	s.mux.Unlock()
	return nil, nil
}

func (s *expectDeleteRecordService) Get(zone, domain, t string) (*dns.Record, *http.Response, error) {
	return nil, nil, errors.New("Expected Record Update, got Get")
}

// expectErrorRecordService fulfils the recordService interface for mocking calls to
// to ns1-go RecordService that will return an error
type expectErrorRecordService struct {
	errorToReturn error
	callCount     int
	mux           *sync.Mutex
}

func (s *expectErrorRecordService) Create(r *dns.Record) (*http.Response, error) {
	s.mux.Lock()
	s.callCount++
	s.mux.Unlock()
	var e error
	if s.errorToReturn != nil {
		e = s.errorToReturn
	} else {
		e = errors.New("default error type")
	}
	return nil, e
}

func (s *expectErrorRecordService) Update(r *dns.Record) (*http.Response, error) {
	s.mux.Lock()
	s.callCount++
	s.mux.Unlock()
	var e error
	if s.errorToReturn != nil {
		e = s.errorToReturn
	} else {
		e = errors.New("default error type")
	}
	return nil, e
}

func (s *expectErrorRecordService) Delete(zone string, domain string, t string) (*http.Response, error) {
	s.mux.Lock()
	s.callCount++
	s.mux.Unlock()
	var e error
	if s.errorToReturn != nil {
		e = s.errorToReturn
	} else {
		e = errors.New("default error type")
	}
	return nil, e
}

func (s *expectErrorRecordService) Get(zone, domain, t string) (*dns.Record, *http.Response, error) {
	s.mux.Lock()
	s.callCount++
	s.mux.Unlock()
	var e error
	if s.errorToReturn != nil {
		e = s.errorToReturn
	} else {
		e = errors.New("default error type")
	}
	return nil, nil, e
}

// mockRecordService fulfils the recordService interface for mocking calls to
// to ns1-go RecordService that will create or update records
type mockRecordService struct {
	callCount int
	records   []*dns.Record
	mux       *sync.Mutex
}

func (s *mockRecordService) Create(r *dns.Record) (*http.Response, error) {
	s.mux.Lock()
	s.callCount++
	s.records = append(s.records, r)
	s.mux.Unlock()
	return nil, nil
}

func (s *mockRecordService) Update(r *dns.Record) (*http.Response, error) {
	s.mux.Lock()
	s.callCount++
	s.records = append(s.records, r)
	s.mux.Unlock()
	return nil, nil
}

func (s *mockRecordService) Delete(zone string, domain string, t string) (*http.Response, error) {
	s.callCount++
	return nil, nil
}

func (s *mockRecordService) Get(zone, domain, t string) (*dns.Record, *http.Response, error) {
	s.callCount++
	return nil, nil, nil
}

// testClient configure and returns a ns1 struct for testing.
// logBuf can be used to set a custom buffer for logging output. If nil, logs will be written to stdout
func testClient(logBuf io.Writer) *ns1 {
	if logBuf == nil {
		logBuf = hclog.DefaultOutput
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "ns1",
		Output: logBuf,
		Level:  hclog.Info,
	})

	n := ns1{
		serviceZone: zone{id: "1", name: "test.zone"},
		ns1Prefix:   "",
		dnsTTL:      10,
		log:         logger,
	}

	return &n
}

func TestSetupServiceZone(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &mockRecordService{},
	}
	expected := zone{id: "57d95da659272400013334de", name: "test.zone"}
	if assert.NoError(t, n.setupServiceZone("test.zone")) {
		assert.Equal(t, expected, n.serviceZone)
	}
	assert.Error(t, n.setupServiceZone("wrong.zone"))
}

func TestGetServices(t *testing.T) {
	n := testClient(nil)
	expected := map[string]service{
		"s1": {name: "s1", nodes: map[string]node{"h1": {}}},
		"s2": {name: "s2", nodes: map[string]node{"h1": {}}},
	}
	n.services = expected
	assert.Equal(t, expected, n.getServices())
}

func TestSetServices(t *testing.T) {
	n := testClient(nil)
	expected := map[string]service{
		"s1": {name: "s1", nodes: map[string]node{"h1": {}}},
		"s2": {name: "s2", nodes: map[string]node{"h1": {}}},
	}
	n.setServices(expected)
	assert.Equal(t, expected, n.services)
}

func TestFetch(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &mockRecordService{},
	}
	expected := map[string]service{
		"s1": {
			name: "s1",
			ns1IDs: recordIDs{
				aRecID:   "57d95da659272400013334dd",
				srvRecID: "57d95da659272400013334dc",
			},
			nodes: map[string]node{
				"1.1.1.1": {
					aRecAnswer: "1.1.1.1",
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
		},
		"s2": {
			name: "s2",
			ns1IDs: recordIDs{
				srvRecID: "57d95da659272400013334db",
			},
			nodes: map[string]node{
				"2.2.2.2": {
					srvRecAnswers: map[int]srvAnswer{
						2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
					},
				},
			},
		},
	}
	err := n.fetch()
	if assert.NoError(t, err) {
		assert.Equal(t, expected, n.services)
	}

	n.serviceZone.name = "wrong.zone"
	assert.Error(t, n.fetch())
}

func TestFetchZone(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &mockRecordService{},
	}
	actual, err := n.fetchZone("test.zone")
	if assert.NoError(t, err) {
		assert.Equal(t, "test.zone", actual.Zone)
	}

	actual, err = n.fetchZone("wrong.zone")
	assert.Error(t, err)
}

func TestTransformZone(t *testing.T) {
	n := ns1{}
	expected := zone{id: "57d95da659272400013334de", name: "test.zone"}
	z := &dns.Zone{
		ID:   "57d95da659272400013334de",
		Zone: "test.zone",
	}
	assert.Equal(t, expected, n.transformZone(z))
}

func TestTransformZoneRecords(t *testing.T) {
	// TODO: convert to table test
	n := ns1{serviceZone: zone{id: "1", name: "test.zone"}}
	z := &dns.Zone{
		ID:   "57d95da659272400013334de",
		Zone: "test.zone",
		Records: []*dns.ZoneRecord{
			{
				Domain:   "s1.test.zone",
				ID:       "57d95da659272400013334dd",
				ShortAns: []string{"1.1.1.1"},
				Type:     "A",
				TTL:      1,
			},
			{
				Domain:   "s1.test.zone",
				ID:       "57d95da659272400013334dc",
				ShortAns: []string{"1 1 1 1.1.1.1"},
				Type:     "SRV",
				TTL:      2,
			},
			{
				Domain:   "s2.test.zone",
				ID:       "57d95da659272400013334db",
				ShortAns: []string{"1 1 2 2.2.2.2"},
				Type:     "SRV",
				TTL:      3,
			},
		},
	}
	expected := map[string]service{
		"s1": {
			name: "s1",
			ns1IDs: recordIDs{
				aRecID:   "57d95da659272400013334dd",
				srvRecID: "57d95da659272400013334dc",
			},
			ttls: recordTTLs{
				aRecTTL:   1,
				srvRecTTL: 2,
			},
			nodes: map[string]node{
				"1.1.1.1": {
					aRecAnswer: "1.1.1.1",
					srvRecAnswers: map[int]srvAnswer{
						1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
					},
				},
			},
		},
		"s2": {
			name: "s2",
			ns1IDs: recordIDs{
				srvRecID: "57d95da659272400013334db",
			},
			ttls: recordTTLs{
				srvRecTTL: 3,
			},
			nodes: map[string]node{
				"2.2.2.2": {
					srvRecAnswers: map[int]srvAnswer{
						2: srvAnswer{priority: 1, weight: 1, port: 2, address: "2.2.2.2"},
					},
				},
			},
		},
	}
	assert.Equal(t, expected, n.transformZoneRecords(z))
}

func TestUpsertRecord(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &expectCreateRecordService{},
	}
	r := &dns.Record{
		Zone:   "test.zone",
		Domain: "s1.test.zone",
		Type:   "A",
		Answers: []*dns.Answer{
			{Rdata: []string{"1.1.1.1"}},
		},
	}

	assert.NoError(t, n.upsertRecord("", r), "Expected Record Create function to be called")
	assert.Equal(t, 1, n.client.Records.(*expectCreateRecordService).callCount, "Expected Record Create function to be called once")

	n.client.Records = &expectUpdateRecordService{}
	assert.NoError(t, n.upsertRecord("id", r), "Expected Record Update function to be called")
	assert.Equal(t, 1, n.client.Records.(*expectUpdateRecordService).callCount, "Expected Record Update function to be called once")

	n.client.Records = &expectErrorRecordService{}
	n.client.Records.(*expectErrorRecordService).mux = &sync.Mutex{}
	assert.EqualError(t, n.upsertRecord("", r), "default error type")
}

func TestGenerateRecord(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &expectGetRecordService{},
	}
	// Test record with fields fetched from NS1
	expected := &dns.Record{
		Zone:    "test.zone",
		Domain:  "s1.test.zone",
		Type:    "A",
		Answers: []*dns.Answer{},
		TTL:     10,
		Filters: []*filter.Filter{
			{Type: "shuffle", Config: filter.Config{}},
		},
	}
	r, err := n.generateRecord("1", "s1", "A")
	assert.NoError(t, err, "Expected Record Get function to be called")
	assert.Equal(t, expected, r)
	assert.Equal(t, 1, n.client.Records.(*expectGetRecordService).callCount, "Expected Record Get function to be called once")
	// Test record with fields generated
	n.client.Records = &expectErrorRecordService{}
	n.client.Records.(*expectErrorRecordService).mux = &sync.Mutex{}
	expected = &dns.Record{
		Zone:    "test.zone",
		Domain:  "s1.test.zone",
		Type:    "A",
		Answers: []*dns.Answer{},
		TTL:     10,
		Meta:    &data.Meta{},
		Filters: []*filter.Filter{},
		Regions: data.Regions{},
	}
	r, err = n.generateRecord("", "s1", "A")
	assert.NoError(t, err, "Expected no Record function to be called")
	assert.Equal(t, expected, r)
	// Test record with error on Get
	r, err = n.generateRecord("1", "s1", "A")
	assert.Error(t, err, "Expected error on call to Record Get function")
}

func TestCreate(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &mockRecordService{},
	}
	n.client.Records.(*mockRecordService).mux = &sync.Mutex{}
	type variant struct {
		input           map[string]service
		expectedRecords []*dns.Record
		expectedCount   int32
	}
	table := map[string]variant{
		"no services": {
			input:           map[string]service{},
			expectedRecords: nil,
			expectedCount:   0,
		},
		"service with no nodes": {
			input:           map[string]service{"s1": {}},
			expectedRecords: []*dns.Record{newTestRecord("A", "s1", n.serviceZone.name, nil), newTestRecord("SRV", "s1", n.serviceZone.name, nil)},
			expectedCount:   2,
		},
		"service with one node": {
			input:           map[string]service{"s2": {nodes: map[string]node{"h1": {}}}},
			expectedRecords: []*dns.Record{newTestRecord("A", "s2", n.serviceZone.name, nil), newTestRecord("SRV", "s2", n.serviceZone.name, nil)},
			expectedCount:   2,
		},
		"multiple services with one node": {
			input: map[string]service{
				"s3": {nodes: map[string]node{"h1": {}}},
				"s4": {nodes: map[string]node{"h2": {}}},
			},
			expectedRecords: []*dns.Record{newTestRecord("A", "s3", n.serviceZone.name, nil), newTestRecord("SRV", "s3", n.serviceZone.name, nil), newTestRecord("A", "s4", n.serviceZone.name, nil), newTestRecord("SRV", "s4", n.serviceZone.name, nil)},
			expectedCount:   4,
		},
		"service with one A rec answer": {
			input: map[string]service{
				"s5": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
			expectedRecords: []*dns.Record{newTestRecord("A", "s5", n.serviceZone.name, []string{"1.1.1.1"}), newTestRecord("SRV", "s5", n.serviceZone.name, nil)},
			expectedCount:   2,
		},
		// not needed with srv type
		/*"service with malformed SRV rec answer": {
			input: map[string]service{
				"s6": {nodes: map[string]node{"h1": {srvRecAnswers: map[int]string{1: "1.1.1.1"}}}},
			},
			expectedRecords: []*dns.Record{newTestRecord("A", "s6", n.serviceZone.name, nil), newTestRecord("SRV", "s6", n.serviceZone.name, nil)},
			expectedCount:   2,
		},*/
		"service with one A and SRV rec answers": {

			input: map[string]service{
				"s7": {
					nodes: map[string]node{
						"h1": {
							aRecAnswer: "1.1.1.1",
							srvRecAnswers: map[int]srvAnswer{
								1: srvAnswer{priority: 1, weight: 1, port: 1, address: "1.1.1.1"},
							},
						},
					},
				},
			},
			expectedRecords: []*dns.Record{newTestRecord("A", "s7", n.serviceZone.name, []string{"1.1.1.1"}), newTestRecord("SRV", "s7", n.serviceZone.name, []string{"1 1 1 1.1.1.1"})},
			expectedCount:   2,
		},
		"service with multiple SRV rec answers": {
			input: map[string]service{
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
			expectedRecords: []*dns.Record{newTestRecord("A", "s8", n.serviceZone.name, nil), newTestRecord("SRV", "s8", n.serviceZone.name, []string{"1 1 1 1.1.1.1", "1 1 2 2.2.2.2"})},
			expectedCount:   2,
		},
		"multiple services with one A rec answer": {
			input: map[string]service{
				"s9":  {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
				"s10": {nodes: map[string]node{"h2": {aRecAnswer: "2.2.2.2"}}},
			},
			expectedRecords: []*dns.Record{newTestRecord("A", "s9", n.serviceZone.name, []string{"1.1.1.1"}), newTestRecord("SRV", "s9", n.serviceZone.name, nil), newTestRecord("A", "s10", n.serviceZone.name, []string{"2.2.2.2"}), newTestRecord("SRV", "s10", n.serviceZone.name, nil)},
			expectedCount:   4,
		},
	}
	for name, v := range table {
		assert.Equal(t, v.expectedCount, n.create(v.input), fmt.Sprintf("test case: %s", name))
		if !assert.Len(t, n.client.Records.(*mockRecordService).records, len(v.expectedRecords), "Actual number of records does not match expected") {
			t.Logf("Expected: %#v\nFound: %#v", v.expectedRecords, n.client.Records.(*mockRecordService).records)
		}
		for _, expectedRec := range v.expectedRecords {
			// search for record in actual
			var actualRec *dns.Record
			for _, a := range n.client.Records.(*mockRecordService).records {
				if a.Domain == expectedRec.Domain && a.Type == expectedRec.Type && a.Zone == expectedRec.Zone {
					actualRec = a
				}
			}
			if actualRec == nil {
				t.Fatalf("\"%v\" does not contain \"%v\"", n.client.Records.(*mockRecordService).records, expectedRec)
			} else {
				assert.Len(t, actualRec.Answers, len(expectedRec.Answers), "Actual number of answers does not match expected")
				for _, a := range expectedRec.Answers {
					assert.Contains(t, actualRec.Answers, a, "Expected answer not found in actual answers")
				}
				assert.Equal(t, expectedRec.TTL, actualRec.TTL, "Expected TTL does not match actual")
				assert.Equal(t, expectedRec.Filters, actualRec.Filters, "Expected Filters do not match actual")
				assert.Equal(t, expectedRec.Link, actualRec.Link, "Expected Link does not match actual")
				assert.Equal(t, expectedRec.Meta, actualRec.Meta, "Expected Meta does not match actual")
				assert.Equal(t, expectedRec.Regions, actualRec.Regions, "Expected Regions do not match actual")
				assert.Equal(t, expectedRec.UseClientSubnet, actualRec.UseClientSubnet, "Expected UseClientSubnet does not match actual")
			}

		}
		// reset records for next test
		n.client.Records.(*mockRecordService).records = nil
	}
}

func TestCreate_WithErrors(t *testing.T) {
	var stderr bytes.Buffer
	n := testClient(&stderr)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &expectErrorRecordService{},
	}
	n.client.Records.(*expectErrorRecordService).mux = &sync.Mutex{}

	type variant struct {
		input              map[string]service
		errorToReturn      error
		expectedRecords    []*dns.Record
		expectedCount      int32
		expectedError      string
		expectedErrorCount int
	}

	table := map[string]variant{
		"Record exists": {
			input: map[string]service{
				"s1": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
			errorToReturn:      ns1api.ErrRecordExists,
			expectedRecords:    nil,
			expectedCount:      0,
			expectedErrorCount: 2,
		},
		"Record missing": {
			input: map[string]service{
				"s1": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
			errorToReturn:      ns1api.ErrRecordMissing,
			expectedRecords:    nil,
			expectedCount:      0,
			expectedErrorCount: 2,
		},
		"Unknown error": {
			input: map[string]service{
				"s1": {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
			},
			errorToReturn:      nil,
			expectedRecords:    nil,
			expectedCount:      0,
			expectedErrorCount: 2,
		},
	}
	for name, v := range table {
		n.client.Records.(*expectErrorRecordService).errorToReturn = v.errorToReturn
		assert.Equal(t, v.expectedCount, n.create(v.input), fmt.Sprintf("Test case: %s", name))
		errCount := 0
		errStr := stderr.String()
		errLines := strings.Split(errStr, "\n")
		for _, line := range errLines {
			msg := line[strings.IndexByte(line, ' ')+1:]
			if strings.HasPrefix(msg, "[ERROR]") {
				errCount++
			}
		}
		assert.Equal(t, v.expectedErrorCount, errCount, fmt.Sprintf("Error count does not meet expected for test case: %s", name))
		stderr.Reset()
	}
}

func TestCreate_WithPrefix(t *testing.T) {
	n := testClient(nil)
	n.ns1Prefix = "TestPrefix"
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &mockRecordService{},
	}
	n.client.Records.(*mockRecordService).mux = &sync.Mutex{}
	input := map[string]service{
		"s9":  {nodes: map[string]node{"h1": {aRecAnswer: "1.1.1.1"}}},
		"s10": {nodes: map[string]node{"h2": {aRecAnswer: "2.2.2.2"}}},
	}
	expectedRecords := []*dns.Record{
		newTestRecord("A", "TestPrefixs9", n.serviceZone.name, []string{"1.1.1.1"}),
		newTestRecord("SRV", "TestPrefixs9", n.serviceZone.name, nil),
		newTestRecord("A", "TestPrefixs10", n.serviceZone.name, []string{"2.2.2.2"}),
		newTestRecord("SRV", "TestPrefixs10", n.serviceZone.name, nil),
	}
	expectedCount := int32(4)
	assert.Equal(t, expectedCount, n.create(input))
	assert.Len(t, n.client.Records.(*mockRecordService).records, len(expectedRecords), "Actual number of records does not match expected")

	// Must check contains as order may differ between actual and expected
	for _, actualRec := range n.client.Records.(*mockRecordService).records {
		if !assert.Contains(t, expectedRecords, actualRec, "Record not found in expected records") {
			for _, e := range expectedRecords {
				if e.Domain == actualRec.Domain && e.Type == actualRec.Type {
					t.Logf("Record found in expected, likely answer mismatch.  Expected %v but found %v", e.Answers, actualRec.Answers)
				}
			}
		}
	}
}

func TestRemove(t *testing.T) {
	n := testClient(nil)
	n.client = &ns1APIClient{
		Zones:   &mockZoneService{},
		Records: &expectDeleteRecordService{},
	}
	n.client.Records.(*expectDeleteRecordService).mux = &sync.Mutex{}
	type variant struct {
		input           map[string]service
		mockRecords     []*dns.Record
		expectedRecords []*dns.Record
		expectedCount   int32
	}
	table := map[string]variant{
		"no services": {
			input:           map[string]service{},
			mockRecords:     nil,
			expectedRecords: nil,
			expectedCount:   0,
		},
		"delete one A record": {
			input: map[string]service{
				"s1": {ns1IDs: recordIDs{aRecID: "r1"}},
			},
			mockRecords:     []*dns.Record{newTestRecord("A", "s1", n.serviceZone.name, nil), newTestRecord("SRV", "s1", n.serviceZone.name, nil)},
			expectedRecords: []*dns.Record{newTestRecord("SRV", "s1", n.serviceZone.name, nil)},
			expectedCount:   1,
		},
		"delete one A and SRV record": {
			input: map[string]service{
				"s2": {ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"}},
			},
			mockRecords:     []*dns.Record{newTestRecord("A", "s2", n.serviceZone.name, nil), newTestRecord("SRV", "s2", n.serviceZone.name, nil)},
			expectedRecords: []*dns.Record{},
			expectedCount:   2,
		},
		"delete two services records": {
			input: map[string]service{
				"s3": {ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"}},
				"s4": {ns1IDs: recordIDs{aRecID: "r1", srvRecID: "r2"}},
			},
			mockRecords:     []*dns.Record{newTestRecord("A", "s3", n.serviceZone.name, nil), newTestRecord("SRV", "s3", n.serviceZone.name, nil), newTestRecord("A", "s4", n.serviceZone.name, nil), newTestRecord("SRV", "s4", n.serviceZone.name, nil)},
			expectedRecords: []*dns.Record{},
			expectedCount:   4,
		},
		"delete apex A record": {
			input: map[string]service{
				"test.zone": {ns1IDs: recordIDs{aRecID: "r1"}},
			},
			mockRecords:     []*dns.Record{newTestRecord("A", "", n.serviceZone.name, nil)},
			expectedRecords: []*dns.Record{},
			expectedCount:   1,
		},
	}

	for name, v := range table {
		n.client.Records.(*expectDeleteRecordService).records = v.mockRecords
		assert.Equal(t, v.expectedCount, n.remove(v.input), fmt.Sprintf("test case: %s", name))

		if !assert.Equal(t, v.expectedRecords, n.client.Records.(*expectDeleteRecordService).records, fmt.Sprintf("test case: %s", name)) {
			t.Logf("Remaining records: %v", n.client.Records.(*expectDeleteRecordService).records)
		}

		// reset records for next test
		n.client.Records.(*expectDeleteRecordService).records = nil
	}

}

// newTestRecord takes a record type t, a service name s, a zone z and an array of answer strings and initializes
// a dns.Record with all fields defined with defaults for testing
func newTestRecord(t string, s string, z string, ans []string) *dns.Record {
	domain := ""
	if s != "" {
		domain = s + "." + z
	} else {
		domain = z
	}
	r := dns.Record{
		Type: t,
		TTL:  10,
		Zone: z,
		//Domain:  s + "." + z,
		Domain:  domain,
		Meta:    &data.Meta{},
		Regions: data.Regions{},
		Filters: []*filter.Filter{},
		Answers: []*dns.Answer{},
	}
	for _, a := range ans {
		aFields := strings.Fields(a)
		r.AddAnswer(dns.NewAnswer(aFields))
	}
	return &r
}
