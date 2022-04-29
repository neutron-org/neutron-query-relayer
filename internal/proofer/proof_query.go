package proofer

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types/kv"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"time"

	//abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	libclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

type QueryKeyProofer struct {
	Client *rpchttp.HTTP
	//Client  abcicli.Client
	ChainID string
}

// TODO: use types from repo?
type StorageValue struct {
	Key    []byte
	Value  []byte
	Proofs []crypto.ProofOp // https://github.com/tendermint/tendermint/blob/29ad4dcb3b260ea7762bb307ae397833e1bd360a/proto/tendermint/crypto/proof.pb.go#L211
}

func NewRPCClient(addr string, timeout time.Duration) (*rpchttp.HTTP, error) {
	httpClient, err := libclient.DefaultHTTPClient(addr)
	if err != nil {
		return nil, err
	}
	httpClient.Timeout = timeout
	rpcClient, err := rpchttp.NewWithClient(addr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}
	return rpcClient, nil
}

func NewQueryKeyProofer(addr string, chainId string) (*QueryKeyProofer, error) {
	timeout := time.Second * 10 // TODO: configure
	client, err := NewRPCClient(addr, timeout)

	if err != nil {
		//TODO: log
		return nil, err
	}

	err = client.Start()
	if err != nil {
		fmt.Printf("Could not start http client")
		return nil, err
	}

	proofer := QueryKeyProofer{Client: client, ChainID: chainId}
	fmt.Printf("Created new query proofer")
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
func (cc *QueryKeyProofer) QueryTendermintProof(ctx context.Context, chainID string, height int64, storeKey string, key []byte) (*StorageValue, clienttypes.Height, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the latest state.
	if height != 0 && height <= 2 {
		return nil, clienttypes.Height{}, fmt.Errorf("proof queries at height <= 2 are not supported")
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

	//res, err := cc.Client.QuerySync(req)
	res, err := cc.Client.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	// TODO: delete if we don't need to convert to proofBz
	//merkleProof, err := commitmenttypes.ConvertProofs(res.ProofOps)
	//if err != nil {
	//	return nil, nil, clienttypes.Height{}, err
	//}

	// TODO: is it correct to create it here?
	//interfaceRegistry := types.NewInterfaceRegistry()
	//cdc := codec.NewProtoCodec(interfaceRegistry)

	//proofBz, err := cdc.Marshal(&merkleProof)
	//if err != nil {
	//	return nil, nil, clienttypes.Height{}, err
	//}

	revision := clienttypes.ParseChainID(chainID)
	return &StorageValue{Value: res.Response.Value, Key: key, Proofs: res.Response.ProofOps.Ops}, clienttypes.NewHeight(revision, uint64(res.Response.Height)+1), nil
}

// QueryIterateTendermintProof retrieves proofs for iterable keys
func (cc *QueryKeyProofer) QueryIterateTendermintProof(ctx context.Context, chainID string, height int64, storeKey string, key []byte) ([]StorageValue, clienttypes.Height, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the latest state.
	if height != 0 && height <= 2 {
		return nil, clienttypes.Height{}, fmt.Errorf("proof queries at height <= 2 are not supported")
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

	//res, err := cc.Client.QuerySync(req)
	res, err := cc.Client.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)

	if err != nil {
		return nil, clienttypes.Height{}, err
	}

	var resPairs kv.Pairs
	err = resPairs.Unmarshal(res.Response.Value)
	if err != nil {
		//	TODO
	}

	var result = make([]StorageValue, 0, len(resPairs.Pairs))
	for _, pair := range resPairs.Pairs {
		storageValue, _, err := cc.QueryTendermintProof(ctx, cc.ChainID, height, storeKey, pair.Key)
		if err != nil {
			return nil, clienttypes.Height{}, err
		}

		result = append(result, *storageValue)
	}

	revision := clienttypes.ParseChainID(chainID)
	return result, clienttypes.NewHeight(revision, uint64(res.Response.Height)+1), nil
}
