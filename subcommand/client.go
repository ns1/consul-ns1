package subcommand

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/nsone/consul-ns1/version"
	ns1api "gopkg.in/ns1/ns1-go.v2/rest"
)

// NS1Client returns a client for the NS1 API
func NS1Client(endpoint string, apiKey string, ignoreSSL bool) (*ns1api.Client, error) {
	decos := []func(*ns1api.Client){}

	ua := fmt.Sprintf("consul-ns1-%s", version.GetHumanVersion())
	k := os.Getenv("NS1_APIKEY")
	if apiKey != "" {
		k = apiKey
	}
	if k == "" {
		return nil, errors.New("NS1 API key must be provided via environment variable NS1_APIKEY or -ns1-apikey flag")
	}
	decos = append(decos, ns1api.SetUserAgent(ua), ns1api.SetAPIKey(k))

	if endpoint != "" {
		decos = append(decos, ns1api.SetEndpoint(endpoint))
	}
	httpClient := configureHTTPDoer(ignoreSSL)
	return ns1api.NewClient(httpClient, decos...), nil
}

// configureHTTPDoer configures HTTP client for the NS1 API
func configureHTTPDoer(ignoreSSL bool) *http.Client {
	httpClient := http.DefaultClient
	if ignoreSSL {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient.Transport = tr
	}
	return httpClient
}
