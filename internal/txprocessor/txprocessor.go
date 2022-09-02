package txprocessor

import (
	"context"
	"encoding/hex"
	"fmt"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
	"time"
)

type TXProcessor struct {
	csManager relay.ConsensusManager
	storage   relay.Storage
	submitter relay.Submitter
	clientID  string
	logger    *zap.Logger
}

func NewTxProcessor(
	csManager relay.ConsensusManager,
	storage relay.Storage,
	submitter relay.Submitter,
	clientID string,
	logger *zap.Logger) TXProcessor {
	return TXProcessor{
		csManager: csManager,
		storage:   storage,
		submitter: submitter,
		clientID:  clientID,
		logger:    logger,
	}
}

func (r TXProcessor) SubmitBlock(ctx context.Context, queryID uint64, block *neutrontypes.Block) error {
	proofStart := time.Now()
	hash := hex.EncodeToString(tmtypes.Tx(block.Tx.Data).Hash())
	neutronTxHash, err := r.submitter.SubmitTxProof(ctx, queryID, r.clientID, block)
	if err != nil {
		neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())
		errSetStatus := r.storage.SetTxStatus(queryID, hash, neutronTxHash, err.Error())
		if errSetStatus != nil {
			return fmt.Errorf("failed to store tx: %w", errSetStatus)
		}
		return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", neutrontypes.InterchainQueryTypeTX, queryID, err)
	}

	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())
	err = r.storage.SetTxStatus(queryID, hash, neutronTxHash, relay.Success)
	if err != nil {
		return fmt.Errorf("failed to store tx: %w", err)
	}
	r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))
	return nil
}

func (r TXProcessor) txToBlock(ctx context.Context, tx relay.Transaction) (*neutrontypes.Block, error) {
	packedHeader, packedNextHeader, err := r.prepareHeaders(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare headers: %w", err)
	}
	block := neutrontypes.Block{
		Header:          packedHeader,
		NextBlockHeader: packedNextHeader,
		Tx:              tx.Tx,
	}
	return &block, nil
}

func (r TXProcessor) ProcessAndSubmit(ctx context.Context, queryID uint64, txs <-chan relay.Transaction) (uint64, error) {
	lastProcessedHeight := uint64(0)
	for tx := range txs {
		if tx.Height > lastProcessedHeight {
			err := r.storage.SetLastQueryHeight(queryID, lastProcessedHeight)
			if err != nil {
				return 0, fmt.Errorf("failed to save last height of query: %w", err)
			}
		}
		lastProcessedHeight = tx.Height
		hash := hex.EncodeToString(tmtypes.Tx(tx.Tx.Data).Hash())
		txExists, err := r.storage.TxExists(queryID, hash)
		if err != nil {
			return 0, fmt.Errorf("failed to check tx existence: %w", err)
		}

		if txExists {
			r.logger.Debug("transaction already submitted", zap.Uint64("query_id", queryID), zap.String("hash", hash))
			continue
		}

		block, err := r.txToBlock(ctx, tx)
		if err != nil {
			return 0, fmt.Errorf("failed to prepsre block: %w", err)
		}

		err = r.SubmitBlock(ctx, queryID, block)
		if err != nil {
			return 0, fmt.Errorf("failed to submit block: %w", err)
		}
	}
	return lastProcessedHeight, nil
}

func (r TXProcessor) prepareHeaders(ctx context.Context, txStruct relay.Transaction) (packedHeader *codectypes.Any, packedNextHeader *codectypes.Any, err error) {
	header, err := r.csManager.GetHeaderWithBestTrustedHeight(ctx, txStruct.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get header for src chain: %w", err)
	}

	packedHeader, err = clienttypes.PackHeader(header)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pack header: %w", err)
	}

	nextHeader, err := r.csManager.GetHeaderWithBestTrustedHeight(ctx, txStruct.Height+1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get next header for src chain: %w", err)
	}

	packedNextHeader, err = clienttypes.PackHeader(nextHeader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pack next header: %w", err)
	}
	return
}
