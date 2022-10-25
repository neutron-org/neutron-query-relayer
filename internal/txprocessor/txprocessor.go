package txprocessor

import (
	"encoding/hex"
	"fmt"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

type TXProcessor struct {
	trustedHeaderFetcher relay.TrustedHeaderFetcher
	storage              relay.Storage
	submitter            relay.Submitter
	logger               *zap.Logger
	dequeue              chan relay.PendingSubmittedTxInfo
}

func NewTxProcessor(
	trustedHeaderFetcher relay.TrustedHeaderFetcher,
	storage relay.Storage,
	submitter relay.Submitter,
	logger *zap.Logger) TXProcessor {
	txProcessor := TXProcessor{
		trustedHeaderFetcher: trustedHeaderFetcher,
		storage:              storage,
		submitter:            submitter,
		logger:               logger,
	}
	txProcessor.dequeue = make(chan relay.PendingSubmittedTxInfo)
	return txProcessor
}

func (r TXProcessor) ProcessAndSubmit(queryID uint64, tx relay.Transaction) error {
	hash := hex.EncodeToString(tmtypes.Tx(tx.Tx.Data).Hash())
	txExists, err := r.storage.TxExists(queryID, hash)
	if err != nil {
		return fmt.Errorf("failed to check tx existence: %w", err)
	}

	if txExists {
		r.logger.Debug("transaction already submitted", zap.Uint64("query_id", queryID), zap.String("hash", hash), zap.Uint64("height", tx.Height))
		return nil
	}

	block, err := r.txToBlock(tx)
	if err != nil {
		return fmt.Errorf("failed to prepare block: %w", err)
	}

	err = r.submitTxWithProofs(queryID, block)
	if err != nil {
		return fmt.Errorf("failed to submit block: %w", err)
	}
	return nil
}

func (r TXProcessor) GetSubmitNotificationChannel() <-chan relay.PendingSubmittedTxInfo {
	return r.dequeue
}

func (r TXProcessor) submitTxWithProofs(queryID uint64, block *neutrontypes.Block) error {
	proofStart := time.Now()
	hash := hex.EncodeToString(tmtypes.Tx(block.Tx.Data).Hash())
	neutronTxHash, err := r.submitter.SubmitTxProof(queryID, block)
	if err != nil {
		neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())
		errSetStatus := r.storage.SetTxStatus(queryID, hash, neutronTxHash, relay.SubmittedTxInfo{Status: relay.ErrorOnSubmit, Message: err.Error()})
		if errSetStatus != nil {
			return fmt.Errorf("failed to store tx: %w", errSetStatus)
		}
		return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", neutrontypes.InterchainQueryTypeTX, queryID, err)
	}

	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())
	err = r.storage.SetTxStatus(queryID, hash, neutronTxHash, relay.SubmittedTxInfo{
		Status: relay.Submitted,
	})
	if err != nil {
		return fmt.Errorf("failed to store tx: %w", err)
	}
	r.dequeue <- relay.PendingSubmittedTxInfo{
		QueryID:         queryID,
		SubmittedTxHash: hash,
		NeutronHash:     neutronTxHash,
		SubmitTime:      time.Now(),
	}
	r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))
	return nil
}

func (r TXProcessor) txToBlock(tx relay.Transaction) (*neutrontypes.Block, error) {
	packedHeader, packedNextHeader, err := r.prepareHeaders(tx)
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

func (r TXProcessor) prepareHeaders(txStruct relay.Transaction) (packedHeader *codectypes.Any, packedNextHeader *codectypes.Any, err error) {
	packedHeader, packedNextHeader, err = r.trustedHeaderFetcher.Fetch(txStruct.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get header for src chain: %w", err)
	}

	return
}
