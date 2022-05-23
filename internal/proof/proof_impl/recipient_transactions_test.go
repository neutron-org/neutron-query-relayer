package proof_impl

import (
	"fmt"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/state"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// VerifyProof checks that TODO
func VerifyProof(results *coretypes.ResultBlockResults, proof merkle.Proof, txIndexInBlock uint32) error {
	//block, err := querier.Client.Block(ctx, &height)
	//if err != nil {
	//	//	TODO: handle error
	//	log.Printf("error fetching block info for height %d: %s", height, err)
	//	return
	//}

	rootHash := abciResponsesResultHash(&state.ABCIResponses{DeliverTxs: results.TxsResults})

	//if !bytes.Equal(rootHash, block.Block.Header.LastResultsHash.Bytes()) {
	//	log.Fatalf("LastResultsHash from block header and calculated LastResultsHash are not equal!")
	//}

	newResults := types.NewResults(results.TxsResults)
	leaf, err := toByteSlice(newResults[txIndexInBlock])
	if err != nil {
		return fmt.Errorf("could not convert recipient transaction with txIndexInBlock=%d into byte slice: %w", txIndexInBlock, err)
	}
	return proof.Verify(rootHash, leaf)
}

func toByteSlice(r *abci.ResponseDeliverTx) ([]byte, error) {
	bz, err := r.Marshal()
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func abciResponsesResultHash(ar *state.ABCIResponses) []byte {
	return types.NewResults(ar.DeliverTxs).Hash()
}
