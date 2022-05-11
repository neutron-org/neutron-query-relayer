package proofs

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

var perPage = 100

// TODO: query transactions once
func ProofTransactions(ctx context.Context, querier *proofer.ProofQuerier, query string) ([]*proofer.CompleteTransactionProof, error) {
	orderBy := ""
	page := 1
	// TODO: pagination support
	searchResult, err := querier.Client.TxSearch(ctx, query, true, &page, &perPage, orderBy)
	fmt.Printf("TxSearch: %+v\n", searchResult)
	if err != nil {
		return nil, fmt.Errorf("could not query new transactions to proof: %w", err)
	}

	if searchResult.TotalCount == 0 {
		// TODO: correct?
		return []*proofer.CompleteTransactionProof{}, nil
	}

	result := make([]*proofer.CompleteTransactionProof, 0, len(searchResult.Txs))
	for _, item := range searchResult.Txs {
		txResultProof, err := TxCompletedSuccessfullyProof(ctx, querier, item.Height, item.Index)
		if err != nil {
			return nil, fmt.Errorf("could not proof transaction with hash=%s: %w", item.Tx.String(), err)
		}

		proof := proofer.CompleteTransactionProof{
			BlockProof:   item.Proof,
			SuccessProof: *txResultProof,
		}
		fmt.Printf("made proof for height=%d index=%d proof=%+v\n", item.Height, item.Index, proof)
		result = append(result, &proof)
	}

	return result, nil
}

func TxCompletedSuccessfullyProof(ctx context.Context, querier *proofer.ProofQuerier, blockHeight int64, txIndexInBlock uint32) (*merkle.Proof, error) {
	results, err := querier.Client.BlockResults(ctx, &blockHeight)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch block results for height = %d: %w", blockHeight, err)
	}

	abciResults := types.NewResults(results.TxsResults)
	proof := abciResults.ProveResult(int(txIndexInBlock))

	return &proof, nil
}

// VerifyProof checks that TODO
func VerifyProof(results *coretypes.ResultBlockResults, proof merkle.Proof, txIndexInBlock uint32) error {
	//block, err := querier.Client.Block(ctx, &height)
	//if err != nil {
	//	//	TODO: handle error
	//	log.Printf("error fetching block info for height %d: %s", height, err)
	//	return
	//}

	rootHash := ABCIResponsesResultsHash(&state.ABCIResponses{DeliverTxs: results.TxsResults})

	//if !bytes.Equal(rootHash, block.Block.Header.LastResultsHash.Bytes()) {
	//	log.Fatalf("LastResultsHash from block header and calculated LastResultsHash are not equal!")
	//}

	newResults := types.NewResults(results.TxsResults)
	leaf, err := toByteSlice(newResults[txIndexInBlock])
	if err != nil {
		// TODO: log cannot convert to byte slice error
		return err
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

func ABCIResponsesResultsHash(ar *state.ABCIResponses) []byte {
	return types.NewResults(ar.DeliverTxs).Hash()
}
