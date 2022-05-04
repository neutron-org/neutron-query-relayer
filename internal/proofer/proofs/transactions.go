package proofs

import (
	"context"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
	"log"
)

var perPage = 100

func ProofTransactions(ctx context.Context, querier *proofer.ProofQuerier, address string) error {
	//{"transfer.recipient": interchain_account}
	query := ""
	orderBy := ""
	//page := 0
	// TODO: pagination support
	searchResult, err := querier.Client.TxSearch(ctx, query, true, nil, &perPage, orderBy)

	// TODO: Calculate height as the height of returned transaction?
	//heightResults := searchResult.Txs[0].Height - 1

	if searchResult.TotalCount == 0 {
		// TODO: correct?
		return nil
	}

	foundTxs := make([]*coretypes.ResultTx, 0, len(searchResult.Txs))
	for _, item := range searchResult.Txs {
		foundTxs = append(foundTxs, item)
	}

	if err != nil {
		return err
	}

	// To verify received transactions we need two root hashes of Merkle trees:
	// 1. **block.Header.DataHash** (MerkleRoot of the hash of transactions) -
	// to verify that transaction is really included in particular block.
	// A relayer can get a proof for a transaction via `Tx RPC query`
	// ([example](https://terra-rpc.easy2stake.com/tx?hash=0xE71F89160178AE8A6AC84F6F8810658CEDF4A66FACA27BA2FFFF2DA8539DE4A6&prove=true));
	// 2. **block.Header.LastResultsHash**
	// (the root hash of a Merkle tree built from `ResponseDeliverTx` responses
	// (`Log`,`Info`, `Codespace` and  `Events` fields are ignored)) -
	// we need to verify that transaction was executed successfully,
	// so we check that `ResponseDeliverTx.Code==0` (meaning no error occured)
	// and verify a response with the Merkle Root.
	// There is no easy way to get proofs for `ResponseDeliverTx` via RPC,
	// so we need a little hack here: a relayer get all `ResponseDeliverTxs`
	// for a particular block via
	// [BlockResults RPC query](https://terra-rpc.easy2stake.com/block_results?height=7411854),
	// and calculates a MerkleProof for an interested transaction by itself
	// (a small [gist example](https://gist.github.com/pr0n00gler/fafd115ae6f46d5a3ee8d2c72f1bf50d));
	//for _, item := range foundTxs {
	//value, err := querier.QueryTxProof(ctx, item.Height, item.Hash)
	//if err != nil {
	// // TODO: handle err
	//return err
	//}

	//???
	//}

	return nil
}

func TxCompletedSuccessfullyProof(ctx context.Context, querier *proofer.ProofQuerier, height int64) {
	heightResults := height - 1

	//block, err := querier.Client.Block(ctx, &height)
	//if err != nil {
	//	//	TODO: handle error
	//	log.Printf("error fetching block info for height %d: %s", height, err)
	//	return
	//}

	results, err := querier.Client.BlockResults(ctx, &heightResults)

	//if !bytes.Equal(rootHash, block.Block.Header.LastResultsHash.Bytes()) {
	//	log.Fatalf("LastResultsHash from block header and calculated LastResultsHash are not equal!")
	//}

	err = VerifyProof(results)
	if err != nil {
		log.Fatalf("failed to verify proof: %v", err)
	}

	log.Println("Proof verified")
}

func VerifyProof(results *coretypes.ResultBlockResults) error {
	rootHash := ABCIResponsesResultsHash(&state.ABCIResponses{DeliverTxs: results.TxsResults})
	proof := types.NewResults(results.TxsResults).ProveResult(0)
	newResults := types.NewResults(results.TxsResults)
	leaf, err := ToByteSlice(newResults[0])
	if err != nil {
		// TODO: log cannot convert to byte slice error
		return err
	}
	return proof.Verify(rootHash, leaf)
}

func ToByteSlices(a types.ABCIResults) [][]byte {
	l := len(a)
	bzs := make([][]byte, l)
	for i := 0; i < l; i++ {

	}
	return bzs
}

func ToByteSlice(r *abci.ResponseDeliverTx) ([]byte, error) {
	bz, err := r.Marshal()
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func ABCIResponsesResultsHash(ar *state.ABCIResponses) []byte {
	return types.NewResults(ar.DeliverTxs).Hash()
}
