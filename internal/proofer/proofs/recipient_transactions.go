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
	"strings"
)

var perPage = 100

const orderBy = ""

// RecipientTransactions gets proofs for query type = 'x/tx/RecipientTransactions'
// (NOTE: there is no such query function in cosmos-sdk)
func (p ProoferImpl) RecipientTransactions(ctx context.Context, queryParams map[string]string) ([]proofer.CompleteTransactionProof, error) {
	query := queryFromParams(queryParams)
	page := 1 // NOTE: page index starts from 1

	txs := make([]*coretypes.ResultTx, 0)
	for {
		searchResult, err := p.querier.Client.TxSearch(ctx, query, true, &page, &perPage, orderBy)
		//fmt.Printf("TxSearch: %+v\n", searchResult)
		if err != nil {
			return nil, fmt.Errorf("could not query new transactions to proof: %w", err)
		}

		if len(searchResult.Txs) == 0 {
			break
		}

		if page*perPage >= searchResult.TotalCount {
			break
		}

		for _, item := range searchResult.Txs {
			txs = append(txs, item)
		}

		page += 1
	}

	if len(txs) == 0 {
		return []proofer.CompleteTransactionProof{}, nil
	}

	result := make([]proofer.CompleteTransactionProof, 0, len(txs))
	for _, item := range txs {
		txResultProof, err := TxCompletedSuccessfullyProof(ctx, p.querier, item.Height, item.Index)
		if err != nil {
			return nil, fmt.Errorf("could not proof transaction with hash=%s: %w", item.Tx.String(), err)
		}

		proof := proofer.CompleteTransactionProof{
			BlockProof:   item.Proof,
			SuccessProof: *txResultProof,
			Height:       uint64(item.Height),
		}
		//fmt.Printf("made proof for height=%d index=%d proof=%+v\n", item.Height, item.Index, proof)
		result = append(result, proof)
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

	rootHash := abciResponsesResultHash(&state.ABCIResponses{DeliverTxs: results.TxsResults})

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

func abciResponsesResultHash(ar *state.ABCIResponses) []byte {
	return types.NewResults(ar.DeliverTxs).Hash()
}

func queryFromParams(params map[string]string) string {
	queryParamsList := make([]string, 0, len(params))
	for key, value := range params {
		queryParamsList = append(queryParamsList, fmt.Sprintf("%s='%s'", key, value))
	}
	return strings.Join(queryParamsList, " AND ")
}
