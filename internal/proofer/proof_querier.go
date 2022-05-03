package proofer

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types/kv"

	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

type ProofQuerier struct {
	Client  *rpcclienthttp.HTTP
	ChainID string
}

func NewProofQuerier(addr string, chainId string) (*ProofQuerier, error) {
	timeout := time.Second * 10 // TODO: configure
	client, err := newRPCClient(addr, timeout)

	if err != nil {
		//TODO: log
		return nil, err
	}

	err = client.Start()
	if err != nil {
		fmt.Printf("Could not start http client")
		return nil, err
	}

	proofer := ProofQuerier{Client: client, ChainID: chainId}
	return &proofer, nil
}

// QueryTendermintProof performs an ABCI query with the given key and returns
// the value of the query, the proto encoded merkle proof, and the height of
// the Tendermint block containing the state root. The desired tendermint height
// to perform the query should be set in the client context. The query will be
// performed at one below this height (at the IAVL version) in order to obtain
// the correct merkle proof. Proof queries at height less than or equal to 2 are
// not supported. Queries with a client context height of 0 will perform a query
// at the latest state available.
// Issue: https://github.com/cosmos/cosmos-sdk/issues/6567
func (cc *ProofQuerier) QueryTendermintProof(ctx context.Context, height int64, storeKey string, key []byte) (*StorageValue, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the latest state.
	if height != 0 && height <= 2 {
		return nil, fmt.Errorf("proof queries at height <= 2 are not supported")
	}

	// Use the IAVL height if a valid tendermint height is passed in.
	// A height of 0 will query with the latest state.
	if height != 0 {
		height--
	}

	req := abci.RequestQuery{
		Path:   fmt.Sprintf("store/%s/key", storeKey),
		Height: height,
		Data:   key,
		Prove:  true,
	}

	opts := rpcclient.ABCIQueryOptions{
		Height: height,
		Prove:  true,
	}

	res, err := cc.Client.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return nil, err
	}

	//revision := clienttypes.ParseChainID(chainID)
	return &StorageValue{Value: res.Response.Value, Key: key, Proofs: res.Response.ProofOps.Ops}, nil
}

// QueryIterateTendermintProof retrieves proofs for subspace of keys
func (cc *ProofQuerier) QueryIterateTendermintProof(ctx context.Context, height int64, storeKey string, key []byte) ([]StorageValue, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the latest state.
	if height != 0 && height <= 2 {
		return nil, fmt.Errorf("proof queries at height <= 2 are not supported")
	}

	// Use the IAVL height if a valid tendermint height is passed in.
	// A height of 0 will query with the latest state.
	if height != 0 {
		height--
	}

	req := abci.RequestQuery{
		Path:   fmt.Sprintf("store/%s/subspace", storeKey),
		Height: height,
		Data:   key,
		Prove:  true,
	}
	opts := rpcclient.ABCIQueryOptions{
		Height: height,
		Prove:  true,
	}

	res, err := cc.Client.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)

	if err != nil {
		return nil, err
	}

	var resPairs kv.Pairs
	err = resPairs.Unmarshal(res.Response.Value)
	if err != nil {
		//	TODO
	}

	var result = make([]StorageValue, 0, len(resPairs.Pairs))
	for _, pair := range resPairs.Pairs {
		storageValue, err := cc.QueryTendermintProof(ctx, height, storeKey, pair.Key)
		if err != nil {
			return nil, err
		}

		result = append(result, *storageValue)
	}

	return result, nil
}

func newRPCClient(addr string, timeout time.Duration) (*rpcclienthttp.HTTP, error) {
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
