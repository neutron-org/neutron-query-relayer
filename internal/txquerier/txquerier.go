package txquerier

import (
	"context"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	"github.com/cometbft/cometbft/types"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

var TxsChanSize = 100

var perPage = 100

const orderBy = "asc"

func NewTXQuerySrv(chainClient relay.ChainClient) *TXQuerierSrv {
	return &TXQuerierSrv{
		chainClient: chainClient,
	}
}

// TXQuerierSrv implementation of relay.TXQuerier interface
type TXQuerierSrv struct {
	chainClient relay.ChainClient
}

// SearchTransactions gets txs with proofs for query type = 'tx'
// (NOTE: there is no such query function in cosmos-sdk)
func (t *TXQuerierSrv) SearchTransactions(ctx context.Context, query string) (<-chan relay.Transaction, <-chan error) {
	errs := make(chan error, 1)
	txs := make(chan relay.Transaction, TxsChanSize)
	page := 1 // NOTE: page index starts from 1

	go func() {
		defer close(txs)
		defer close(errs)
		for {
			searchResult, err := t.chainClient.TxSearch(ctx, query, true, &page, &perPage, orderBy)
			if err != nil {
				errs <- fmt.Errorf("could not query new transactions to proof: %w", err)
				return
			}

			if len(searchResult.Txs) == 0 {
				return
			}

			for _, tx := range searchResult.Txs {
				deliveryProof, deliveryResult, err := t.proofDelivery(ctx, tx.Height, tx.Index)
				if err != nil {
					errs <- fmt.Errorf("could not proof transaction with hash=%s: %w", tx.Tx.String(), err)
					return
				}

				txProof := neutrontypes.TxValue{
					InclusionProof: cryptoProofFromMerkleProof(tx.Proof.Proof),
					DeliveryProof:  deliveryProof,
					Response:       deliveryResult,
					Data:           tx.Tx,
				}
				select {
				case txs <- relay.Transaction{Tx: &txProof, Height: uint64(tx.Height)}:
				case <-ctx.Done():
					return
				}
			}

			if page*perPage >= searchResult.TotalCount {
				return
			}

			page += 1
		}
	}()

	return txs, errs
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

func cryptoProofFromMerkleProof(mp merkle.Proof) *crypto.Proof {
	cp := new(crypto.Proof)

	cp.Total = mp.Total
	cp.Index = mp.Index
	cp.LeafHash = mp.LeafHash
	cp.Aunts = mp.Aunts

	return cp
}
