package subcommand

import (
	"crypto/tls"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	ns1api "gopkg.in/ns1/ns1-go.v2/rest"
)

func UnsetEnv() (restore func()) {
	before := os.Getenv("NS1_APIKEY")
	os.Unsetenv("NS1_APIKEY")

	return func() {
		os.Setenv("NS1_APIKEY", before)
	}
}

func TestNS1Client_APIKey(t *testing.T) {
	defer UnsetEnv()()
	k := "testapikey"
	os.Setenv("NS1_APIKEY", k)
	expected := ns1api.NewClient(nil, ns1api.SetAPIKey(k))
	client, err := NS1Client("", "", false)
	if assert.NoError(t, err) {
		assert.Equal(t, expected.APIKey, client.APIKey)
	}

	k = "testanotherapikey"
	expected = ns1api.NewClient(nil, ns1api.SetAPIKey(k))
	client, err = NS1Client("", k, false)
	if assert.NoError(t, err) {
		assert.Equal(t, expected.APIKey, client.APIKey)
	}
}

func TestNS1Client_Endpoint(t *testing.T) {
	k := "testapikey"
	expected := ns1api.NewClient(nil, ns1api.SetAPIKey(k))
	client, err := NS1Client("", k, false)
	if assert.NoError(t, err) {
		assert.Equal(t, expected.Endpoint, client.Endpoint)
		assert.Contains(t, client.UserAgent, "consul-ns1")
	}

	endpoint := "http://ns1.endpoint.test/"
	expected = ns1api.NewClient(nil,
		ns1api.SetAPIKey("testapikey"),
		ns1api.SetEndpoint(endpoint))
	client, err = NS1Client(endpoint, k, false)
	if assert.NoError(t, err) {
		assert.Equal(t, expected.Endpoint, client.Endpoint)
		assert.Contains(t, client.UserAgent, "consul-ns1")
	}
}

func TestNS1Client_Error(t *testing.T) {
	defer UnsetEnv()()
	client, err := NS1Client("http://ns1.endpoint.test/", "", false)
	assert.Nil(t, client)
	assert.Error(t, err)
}

func TestConfigureHTTPDoer(t *testing.T) {
	expected := http.DefaultClient
	assert.Equal(t, expected, configureHTTPDoer(false))

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	expected.Transport = tr
	assert.Equal(t, expected, configureHTTPDoer(true))
}
