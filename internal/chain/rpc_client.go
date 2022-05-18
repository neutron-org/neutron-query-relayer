package chain

import (
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
	rpcClient, err := rpcclienthttp.NewWithClient(addr, httpClient)
	if err != nil {
		return nil, err
	}
	return rpcClient, nil
}
