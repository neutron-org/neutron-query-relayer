package proof_impl

import (
	"context"
	"fmt"
	"strings"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
	"github.com/tendermint/tendermint/types"

	"github.com/neutron-org/cosmos-query-relayer/internal/relay"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

var perPage = 100

const orderBy = "asc"

func cryptoProofFromMerkleProof(mp merkle.Proof) *crypto.Proof {
	cp := new(crypto.Proof)

	cp.Total = mp.Total
	cp.Index = mp.Index
	cp.LeafHash = mp.LeafHash
	cp.Aunts = mp.Aunts

	return cp
}

// SearchTransactions gets proofs for query type = 'tx'
func (p ProoferImpl) SearchTransactions(ctx context.Context, filter neutrontypes.TransactionsFilter) ([]relay.Transaction, error) {
	query, err := constructQuery(filter)
	if err != nil {
		return nil, fmt.Errorf("could not compose query: %v", err)
	}
	page := 1 // NOTE: page index starts from 1

	txs := make([]relay.Transaction, 0)
	for {
		searchResult, err := p.querier.Client.TxSearch(ctx, query, true, &page, &perPage, orderBy)
		if err != nil {
			return nil, fmt.Errorf("could not query new transactions to proof: %w", err)
		}

		if len(searchResult.Txs) == 0 {
			break
		}

		for _, tx := range searchResult.Txs {
			deliveryProof, deliveryResult, err := p.proofDelivery(ctx, tx.Height, tx.Index)
			if err != nil {
				return nil, fmt.Errorf("could not proof transaction with hash=%s: %w", tx.Tx.String(), err)
			}

			txProof := neutrontypes.TxValue{
				InclusionProof: cryptoProofFromMerkleProof(tx.Proof.Proof),
				DeliveryProof:  deliveryProof,
				Response:       deliveryResult,
				Data:           tx.Tx,
			}

			txs = append(txs, relay.Transaction{Tx: &txProof, Height: uint64(tx.Height)})
		}

		if page*perPage >= searchResult.TotalCount {
			break
		}

		page += 1
	}

	return txs, nil
}

// proofDelivery returns (deliveryProof, deliveryResult, error) for transaction in block 'blockHeight' with index 'txIndexInBlock'
func (p ProoferImpl) proofDelivery(ctx context.Context, blockHeight int64, txIndexInBlock uint32) (*crypto.Proof, *abci.ResponseDeliverTx, error) {
	results, err := p.querier.Client.BlockResults(ctx, &blockHeight)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch block results for height = %d: %w", blockHeight, err)
	}

	txsResults := results.TxsResults
	abciResults := types.NewResults(txsResults)
	txProof := abciResults.ProveResult(int(txIndexInBlock))
	txResult := txsResults[txIndexInBlock]

	return cryptoProofFromMerkleProof(txProof), txResult, nil
}

// constructQuery creates query from params like `key1{=,>,>=,<,<=}value1 AND key2{=,>,>=,<,<=}value2 AND ...`
func constructQuery(params neutrontypes.TransactionsFilter) (string, error) {
	queryParamsList := make([]string, 0, len(params))
	for _, row := range params {
		sign, err := getOpSign(row.Op)
		if err != nil {
			return "", err
		}

		var attribute string
		switch r := row.Value.(type) {
		case string:
			attribute = fmt.Sprintf("%s%s'%s'", row.Field, sign, r)
		case float64:
			attribute = fmt.Sprintf("%s%s%d", row.Field, sign, uint64(r))
		case uint64:
			attribute = fmt.Sprintf("%s%s%d", row.Field, sign, r)
		default:
			return "", fmt.Errorf("unsupported row.Value type")
		}

		queryParamsList = append(queryParamsList, attribute)
	}
	return strings.Join(queryParamsList, " AND "), nil
}

func getOpSign(op string) (string, error) {
	switch strings.ToLower(op) {
	case "eq":
		return "=", nil
	case "gt":
		return ">", nil
	case "gte":
		return ">=", nil
	case "lt":
		return "<", nil
	case "lte":
		return "<=", nil
	default:
		return "", fmt.Errorf("unsupported operator %s", op)
	}
}
