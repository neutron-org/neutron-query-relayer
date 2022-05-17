package proofer

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/kv"
	"strings"
	"time"

	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	jsonrpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

type ProofQuerier struct {
	Client  *rpcclienthttp.HTTP
	ChainID string
	cdc     codec.LegacyAmino
}

func NewProofQuerier(timeout time.Duration, addr string, chainId string) (*ProofQuerier, error) {
	client, err := NewRPCClient(addr, timeout)

	if err != nil {
		return nil, fmt.Errorf("could not initialize http client: %w", err)
	}

	err = client.Start()
	if err != nil {
		return nil, fmt.Errorf("could not start http client: %w", err)
	}

	legacyCdc := codec.NewLegacyAmino()
	//cdc := codec.NewAminoCodec(legacyCdc)

	proofer := ProofQuerier{Client: client, ChainID: chainId, cdc: *legacyCdc}
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
func (cc *ProofQuerier) QueryTendermintProof(ctx context.Context, height int64, storeKey string, key []byte) (*StorageValue, uint64, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the latest state.
	if height != 0 && height <= 2 {
		return nil, 0, fmt.Errorf("proof queries at height <= 2 are not supported")
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
		return nil, 0, err
	}

	//revision := clienttypes.ParseChainID(chainID)
	response := res.Response
	// TODO: is storagePrefix correct here?
	return &StorageValue{Value: response.Value, Key: key, Proofs: response.ProofOps.Ops, StoragePrefix: storeKey}, uint64(response.Height), nil
}

// QueryIterateTendermintProof retrieves proofs for subspace of keys
func (cc *ProofQuerier) QueryIterateTendermintProof(ctx context.Context, height int64, storeKey string, key []byte) ([]StorageValue, uint64, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the latest state.
	if height != 0 && height <= 2 {
		return nil, 0, fmt.Errorf("proof queries at height <= 2 are not supported")
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
		return nil, 0, err
	}

	if res.Response.Code != 0 {
		return nil, 0, fmt.Errorf("not zero response code for abci subspace query with code=%s log=%s", res.Response.Code, res.Response.Log)
	}

	var resPairs kv.Pairs
	err = resPairs.Unmarshal(res.Response.Value)
	if err != nil {
		return nil, 0, err
	}

	var result = make([]StorageValue, 0, len(resPairs.Pairs))
	for _, pair := range resPairs.Pairs {
		storageValue, _, err := cc.QueryTendermintProof(ctx, res.Response.Height, storeKey, pair.Key)
		if err != nil {
			return nil, 0, err
		}

		result = append(result, *storageValue)
	}

	return result, uint64(res.Response.Height), nil
}

// TODO: move out of here?
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

func (cc *ProofQuerier) Test(ctx context.Context, validatorAddress []byte, startHeight, endingHeight uint64) error {
	height := int64(0)

	///cosmos.staking.v1beta1.Query/DelegatorDelegations
	// path := fmt.Sprintf("cosmos.distribution.v1beta1.Query/ValidatorSlashes", validatorAddress)
	path := strings.Join([]string{"custom", distributiontypes.QuerierRoute, distributiontypes.QueryValidatorSlashes}, "/")
	params := distributiontypes.NewQueryValidatorSlashesParams(validatorAddress, startHeight, endingHeight)
	bz := cc.cdc.MustMarshalJSON(&params)
	//TODO: marshal params
	var back distributiontypes.QueryValidatorSlashesParams
	cc.cdc.MustUnmarshalJSON(bz, &back)

	req := abci.RequestQuery{
		Path:   path,
		Height: height,
		Data:   bz,
		Prove:  true,
	}
	res, err := cc.Client.ABCIQuery(ctx, req.Path, req.Data)

	fmt.Printf("Result of Test: %+v\n", res)

	return err
}
