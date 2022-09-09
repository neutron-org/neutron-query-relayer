package txquerier

import (
	"context"
	"fmt"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
	"strings"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

var perPage = 100

var TxsChanSize = 100

const orderBy = "asc"

func NewTXQuerySrv(chainClient relay.ChainClient) *TXQuerierSrv {
	return &TXQuerierSrv{
		chainClient: chainClient,
	}
}

// TXQuerierSrv implementation of relay.TXQuerier interface
type TXQuerierSrv struct {
	chainClient relay.ChainClient
	err         error
}

// SearchTransactions gets txs with proofs for query type = 'tx'
// (NOTE: there is no such query function in cosmos-sdk)
func (t *TXQuerierSrv) SearchTransactions(ctx context.Context, txFilter neutrontypes.TransactionsFilter) <-chan relay.Transaction {
	t.err = nil
	txs := make(chan relay.Transaction, TxsChanSize)
	query, err := queryFromTxFilter(txFilter)
	if err != nil {
		t.err = fmt.Errorf("could not compose query: %v", err)
		close(txs)
		return txs
	}
	page := 1 // NOTE: page index starts from 1

	go func() {
		defer close(txs)
		for {
			searchResult, err := t.chainClient.TxSearch(ctx, query, true, &page, &perPage, orderBy)
			if err != nil {
				t.err = fmt.Errorf("could not query new transactions to proof: %w", err)
				return
			}

			if len(searchResult.Txs) == 0 {
				return
			}

			for _, tx := range searchResult.Txs {
				deliveryProof, deliveryResult, err := t.proofDelivery(ctx, tx.Height, tx.Index)
				if err != nil {
					t.err = fmt.Errorf("could not proof transaction with hash=%s: %w", tx.Tx.String(), err)
					return
				}

				txProof := neutrontypes.TxValue{
					InclusionProof: cryptoProofFromMerkleProof(tx.Proof.Proof),
					DeliveryProof:  deliveryProof,
					Response:       deliveryResult,
					Data:           tx.Tx,
				}
				txs <- relay.Transaction{Tx: &txProof, Height: uint64(tx.Height)}
			}

			if page*perPage >= searchResult.TotalCount {
				return
			}

			page += 1
		}
	}()

	return txs
}

func (t *TXQuerierSrv) Err() error {
	return t.err
}

// proofDelivery returns (deliveryProof, deliveryResult, error) for transaction in block 'blockHeight' with index 'txIndexInBlock'
func (t *TXQuerierSrv) proofDelivery(ctx context.Context, blockHeight int64, txIndexInBlock uint32) (*crypto.Proof, *abci.ResponseDeliverTx, error) {
	results, err := t.chainClient.BlockResults(ctx, &blockHeight)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch block results for height = %d: %w", blockHeight, err)
	}

	txsResults := results.TxsResults
	abciResults := types.NewResults(txsResults)
	txProof := abciResults.ProveResult(int(txIndexInBlock))
	txResult := txsResults[txIndexInBlock]

	return cryptoProofFromMerkleProof(txProof), txResult, nil
}

// queryFromTxFilter creates query from transactions filter like
// `key1{=,>,>=,<,<=}value1 AND key2{=,>,>=,<,<=}value2 AND ...`
func queryFromTxFilter(params neutrontypes.TransactionsFilter) (string, error) {
	queryParamsList := make([]string, 0, len(params))
	for _, row := range params {
		sign, err := getOpSign(row.Op)
		if err != nil {
			return "", err
		}
		switch r := row.Value.(type) {
		case string:
			queryParamsList = append(queryParamsList, fmt.Sprintf("%s%s'%s'", row.Field, sign, r))
		case float64:
			queryParamsList = append(queryParamsList, fmt.Sprintf("%s%s%d", row.Field, sign, uint64(r)))
		case uint64:
			queryParamsList = append(queryParamsList, fmt.Sprintf("%s%s%d", row.Field, sign, r))
		}
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

func cryptoProofFromMerkleProof(mp merkle.Proof) *crypto.Proof {
	cp := new(crypto.Proof)

	cp.Total = mp.Total
	cp.Index = mp.Index
	cp.LeafHash = mp.LeafHash
	cp.Aunts = mp.Aunts

	return cp
}
