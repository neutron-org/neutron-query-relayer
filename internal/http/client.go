package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
)

const getTimeout = time.Second * 5

// ICQClient provides high level methods to work with ICQ webserver api
type ICQClient struct {
	host   *url.URL
	client http.Client
}

// NewICQClient takes a host as a single argument and returns an ICQClient in case of well formatted host arg
// host format is <scheme>://<host>[:<port>], e.g. http://myicq.host, https://myicq.host, http://myicq.host:8080
func NewICQClient(host string) (*ICQClient, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("host parsing error: %w", err)
	}

	u.Path = ""
	u.RawQuery = ""
	return &ICQClient{
		host: u,
		client: http.Client{
			Timeout: getTimeout,
		},
	}, nil
}

func (c ICQClient) GetUnsuccessfulTxs() ([]relay.UnsuccessfulTxInfo, error) {
	u := *c.host
	u.Path = UnsuccessfulTxsResource

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build http request: %w", err)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make http request: %w", err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("got unexpected http response status code: %d", res.StatusCode)
	}
	txs := make([]relay.UnsuccessfulTxInfo, 0)

	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&txs)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return txs, nil
}

func (c ICQClient) ResubmitTxs(txs ResubmitRequest) error {
	u := *c.host
	u.Path = ResubmitTxs
	body := bytes.Buffer{}
	encoder := json.NewEncoder(&body)
	err := encoder.Encode(txs)
	if err != nil {
		return fmt.Errorf("failed to marshal txs: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), &body)
	if err != nil {
		return fmt.Errorf("failed to build http request: %w", err)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make http request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 400 {
		errBody := bytes.Buffer{}
		_, err = errBody.ReadFrom(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read response(code 400) body: %w", err)
		}
		return fmt.Errorf(errBody.String())
	} else if res.StatusCode != 200 {
		return fmt.Errorf("got unexpected http response status code: %d", res.StatusCode)
	}

	return nil
}
