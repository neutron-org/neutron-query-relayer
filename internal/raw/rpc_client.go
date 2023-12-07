package raw

import (
	"fmt"
	"time"

	rpcclienthttp "github.com/cometbft/cometbft/rpc/client/http"
	jsonrpcclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
)

const socketEndpoint = "/websocket"

// NewRPCClient returns connected client for RPC queries into blockchain
func NewRPCClient(addr string, timeout time.Duration) (*rpcclienthttp.HTTP, error) {
	httpClient, err := jsonrpcclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, fmt.Errorf("could not create http client with address=%s: %w", addr, err)
	}

	httpClient.Timeout = timeout
	rpcClient, err := rpcclienthttp.NewWithClient(addr, socketEndpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("could not initialize rpc client from http client with address=%s: %w", addr, err)
	}

	err = rpcClient.Start()
	if err != nil {
		return nil, fmt.Errorf("could not start rpc client with address=%s: %w", addr, err)
	}

	return rpcClient, nil
}
