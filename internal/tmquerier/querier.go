package tmquerier

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Querier can get proofs for stored blockchain values
type Querier struct {
	Client                 *rpcclienthttp.HTTP
	ChainID                string
	ValidatorAccountPrefix string
	cdc                    codec.LegacyAmino
}

func NewQuerier(client *rpcclienthttp.HTTP, chainId string, validatorAccountPrefix string) (*Querier, error) {
	legacyCdc := codec.NewLegacyAmino()
	return &Querier{Client: client, ChainID: chainId, cdc: *legacyCdc, ValidatorAccountPrefix: validatorAccountPrefix}, nil
}

// QueryTendermintProof performs an ABCI query with the given key and returns
// the value of the query, the proto encoded merkle proof, and the height of
// the Tendermint block containing the state root. The query will be
// performed at one below this height (at the IAVL version) in order to obtain
// the correct merkle proof. Proof queries at height less than or equal to 2 are
// not supported. Txs with a client context height of 0 will perform a query
// at the latest state available.
// Issue: https://github.com/cosmos/cosmos-sdk/issues/6567
// NOTE: returned uint64 height=(HEIGHT + 1) which is a height of a block with root_hash proofs it, NOT the block number that we got value for
func (q *Querier) QueryTendermintProof(ctx context.Context, height int64, storeKey string, key []byte) (*neutrontypes.StorageValue, uint64, error) {
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

	res, err := q.Client.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("error making abci query for tendermint proof: %w", err)
	}

	response := res.Response
	return &neutrontypes.StorageValue{Value: response.Value, Key: key, Proof: response.ProofOps, StoragePrefix: storeKey}, uint64(response.Height + 1), nil
}
