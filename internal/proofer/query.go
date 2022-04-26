package proofer

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	commitmenttypes "github.com/cosmos/ibc-go/v3/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	lens "github.com/strangelove-ventures/lens/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
	"os"
)

type ProofQueries struct {
	client *lens.ChainClient
	//rpcClient rpcclient.Client
}

//ccc := lens.GetCosmosHubConfig("TODO: keydirectory", false)

func NewProofQueries(log *zap.Logger, homepath string, ccc *lens.ChainClientConfig) (*ProofQueries, error) {
	//pc := nil
	client, err := lens.NewChainClient(
		log.With(zap.String("sys", "chain_client")),
		//lens.ChainClientConfig(&pc),
		//TODO: config
		ccc,
		homepath,
		os.Stdin,
		os.Stdout,
	)

	if err != nil {
		return nil, err
	}

	return &ProofQueries{client}, nil
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
func (cc *ProofQueries) QueryTendermintProof(ctx context.Context, chainID string, height int64, key []byte) ([]byte, []byte, clienttypes.Height, error) {
	// ABCI queries at heights 1, 2 or less than or equal to 0 are not supported.
	// Base app does not support queries for height less than or equal to 1.
	// Therefore, a query at height 2 would be equivalent to a query at height 3.
	// A height of 0 will query with the lastest state.
	if height != 0 && height <= 2 {
		return nil, nil, clienttypes.Height{}, fmt.Errorf("proof queries at height <= 2 are not supported")
	}

	// Use the IAVL height if a valid tendermint height is passed in.
	// A height of 0 will query with the latest state.
	if height != 0 {
		height--
	}

	req := abci.RequestQuery{
		Path:   fmt.Sprintf("store/%s/key", host.StoreKey),
		Height: height,
		Data:   key,
		Prove:  true,
	}

	res, err := cc.client.QueryABCI(ctx, req)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	merkleProof, err := commitmenttypes.ConvertProofs(res.ProofOps)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	// TODO: is it correct to create it here?
	interfaceRegistry := types.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(interfaceRegistry)

	proofBz, err := cdc.Marshal(&merkleProof)
	if err != nil {
		return nil, nil, clienttypes.Height{}, err
	}

	revision := clienttypes.ParseChainID(chainID)
	return res.Value, proofBz, clienttypes.NewHeight(revision, uint64(res.Height)+1), nil
}

//func (cc *ProofQueries) QueryABCI(ctx context.Context, req abci.RequestQuery) (abci.ResponseQuery, error) {
//	opts := rpcclient.ABCIQueryOptions{
//		Height: req.Height,
//		Prove:  req.Prove,
//	}
//	result, err := cc.rpcClient.ABCIQueryWithOptions(ctx, req.Path, req.Data, opts)
//	if err != nil {
//		return abci.ResponseQuery{}, err
//	}
//
//	if !result.Response.IsOK() {
//		return abci.ResponseQuery{}, sdkErrorToGRPCError(result.Response)
//	}
//
//	// data from trusted node or subspace query doesn't need verification
//	if !opts.Prove || !isQueryStoreWithProof(req.Path) {
//		return result.Response, nil
//	}
//
//	return result.Response, nil
//}
//
//func sdkErrorToGRPCError(resp abci.ResponseQuery) error {
//	switch resp.Code {
//	case sdkerrors.ErrInvalidRequest.ABCICode():
//		return status.Error(codes.InvalidArgument, resp.Log)
//	case sdkerrors.ErrUnauthorized.ABCICode():
//		return status.Error(codes.Unauthenticated, resp.Log)
//	case sdkerrors.ErrKeyNotFound.ABCICode():
//		return status.Error(codes.NotFound, resp.Log)
//	default:
//		return status.Error(codes.Unknown, resp.Log)
//	}
//}
//
//// isQueryStoreWithProof expects a format like /<queryType>/<storeName>/<subpath>
//// queryType must be "store" and subpath must be "key" to require a proof.
//func isQueryStoreWithProof(path string) bool {
//	if !strings.HasPrefix(path, "/") {
//		return false
//	}
//
//	paths := strings.SplitN(path[1:], "/", 3)
//
//	switch {
//	case len(paths) != 3:
//		return false
//	case paths[0] != "store":
//		return false
//	case rootmulti.RequireProof("/" + paths[2]):
//		return true
//	}
//
//	return false
//}
