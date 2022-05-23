package raw

import (
	"fmt"
	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"time"
)

func NewRPCClient(addr string, timeout time.Duration) (*rpcclienthttp.HTTP, error) {
	httpClient, err := jsonrpcclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}
	httpClient.Timeout = timeout
	rpcClient, err := rpcclienthttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, fmt.Errorf("could not initialize rpc client from http client: %w", err)
	}

	err = rpcClient.Start()

	if err != nil {
		return nil, fmt.Errorf("could not start rpc client: %w", err)
	}

	return rpcClient, nil
}
