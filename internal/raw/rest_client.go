package raw

import (
	"fmt"
	neturl "net/url"

	restclient "github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client"
)

const restClientBasePath = "/"

// NewRESTClient makes sure that the restAddr is formed correctly and returns a REST query.
func NewRESTClient(restAddr string) (*restclient.HTTPAPIConsole, error) {
	url, err := neturl.Parse(restAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse restAddr: %w", err)
	}

	return restclient.NewHTTPClientWithConfig(nil, &restclient.TransportConfig{
		Host:     url.Host,
		BasePath: restClientBasePath,
		Schemes:  []string{url.Scheme},
	}), nil
}
