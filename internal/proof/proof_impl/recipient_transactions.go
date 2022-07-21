package proof_impl

import (
	"context"
	"fmt"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
	"strings"
)

var perPage = 100

const orderBy = "desc"

func cryptoProofFromMerkleProof(mp merkle.Proof) *crypto.Proof {
	cp := new(crypto.Proof)

	cp.Total = mp.Total
	cp.Index = mp.Index
	cp.LeafHash = mp.LeafHash
	cp.Aunts = mp.Aunts

	return cp
}

// SearchTransactionsWithProofs gets proofs for query type = 'tx'
// (NOTE: there is no such query function in cosmos-sdk)
func (p ProoferImpl) SearchTransactionsWithProofs(ctx context.Context, queryParams map[string]string) (map[uint64][]*neutrontypes.TxValue, error) {
	query := queryFromParams(queryParams)
	page := 1 // NOTE: page index starts from 1

	blocks := make(map[uint64][]*neutrontypes.TxValue, 0)
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

			txs := blocks[uint64(tx.Height)]
			txs = append(txs, &txProof)
			blocks[uint64(tx.Height)] = txs
		}

		if page*perPage >= searchResult.TotalCount {
			break
		}

		page += 1
	}

	return blocks, nil
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

// queryFromParams creates query from params like `key1=value1 AND key2=value2 AND ...`
func queryFromParams(params map[string]string) string {
	queryParamsList := make([]string, 0, len(params))
	for key, value := range params {
		queryParamsList = append(queryParamsList, fmt.Sprintf("%s='%s'", key, value))
	}
	return strings.Join(queryParamsList, " AND ")
}
