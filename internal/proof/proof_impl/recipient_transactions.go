package proof_impl

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
	"strings"
)

var perPage = 100

const orderBy = "desc"

// RecipientTransactions gets proofs for query type = 'x/tx/RecipientTransactions'
// (NOTE: there is no such query function in cosmos-sdk)
func (p ProoferImpl) RecipientTransactions(ctx context.Context, queryParams map[string]string) ([]proof.TxValue, error) {
	query := queryFromParams(queryParams)
	page := 1 // NOTE: page index starts from 1

	txs := make([]*coretypes.ResultTx, 0)
	for {
		searchResult, err := p.querier.Client.TxSearch(ctx, query, true, &page, &perPage, orderBy)
		if err != nil {
			return nil, fmt.Errorf("could not query new transactions to proof: %w", err)
		}

		if len(searchResult.Txs) == 0 {
			break
		}

		if page*perPage >= searchResult.TotalCount {
			break
		}

		for _, tx := range searchResult.Txs {
			txs = append(txs, tx)
		}

		page += 1
	}

	result := make([]proof.TxValue, 0, len(txs))
	for _, tx := range txs {
		deliveryProof, deliveryResult, err := p.txCompletedSuccessfullyProof(ctx, tx.Height, tx.Index)
		inclusionProof := tx.Proof.Proof
		if err != nil {
			return nil, fmt.Errorf("could not proof transaction with hash=%s: %w", tx.Tx.String(), err)
		}

		txProof := proof.TxValue{
			InclusionProof: inclusionProof,
			DeliveryProof:  *deliveryProof,
			Tx:             deliveryResult,
			Height:         uint64(tx.Height),
		}
		result = append(result, txProof)
	}

	return result, nil
}

// txCompletedSuccessfullyProof returns (deliveryProof, deliveryResult, error) for transaction in block 'blockHeight' with index 'txIndexInBlock'
func (p ProoferImpl) txCompletedSuccessfullyProof(ctx context.Context, blockHeight int64, txIndexInBlock uint32) (*merkle.Proof, *abci.ResponseDeliverTx, error) {
	results, err := p.querier.Client.BlockResults(ctx, &blockHeight)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch block results for height = %d: %w", blockHeight, err)
	}

	txsResults := results.TxsResults
	abciResults := types.NewResults(txsResults)
	txProof := abciResults.ProveResult(int(txIndexInBlock))
	txResult := txsResults[txIndexInBlock]

	return &txProof, txResult, nil
}

// queryFromParams creates query from params like `key1=value1 AND key2=value2 AND ...`
func queryFromParams(params map[string]string) string {
	queryParamsList := make([]string, 0, len(params))
	for key, value := range params {
		queryParamsList = append(queryParamsList, fmt.Sprintf("%s='%s'", key, value))
	}
	return strings.Join(queryParamsList, " AND ")
}
