package txprocessor

import (
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/internal/metrics"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

type TXProcessor struct {
	trustedHeaderFetcher        relay.TrustedHeaderFetcher
	storage                     relay.Storage
	submitter                   relay.Submitter
	logger                      *zap.Logger
	checkSubmittedTxStatusDelay time.Duration
	ignoreErrorsRegexp          *regexp.Regexp
}

func NewTxProcessor(
	trustedHeaderFetcher relay.TrustedHeaderFetcher,
	storage relay.Storage,
	submitter relay.Submitter,
	logger *zap.Logger,
	checkSubmittedTxStatusDelay time.Duration,
	ignoreErrorsRegexp string,
) TXProcessor {
	txProcessor := TXProcessor{
		trustedHeaderFetcher:        trustedHeaderFetcher,
		storage:                     storage,
		submitter:                   submitter,
		logger:                      logger,
		checkSubmittedTxStatusDelay: checkSubmittedTxStatusDelay,
		ignoreErrorsRegexp:          regexp.MustCompile(ignoreErrorsRegexp),
	}

	return txProcessor
}

func (r TXProcessor) ProcessAndSubmit(
	ctx context.Context,
	queryID uint64,
	tx relay.Transaction,
	submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo,
) error {
	block, err := r.txToBlock(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to prepare block: %w", err)
	}

	if err = r.submitTxWithProofs(ctx, queryID, tx.Height, block, submittedTxsTasksQueue); err != nil {
		return fmt.Errorf("failed to submit block: %w", err)
	}
	return nil
}

func (r TXProcessor) submitTxWithProofs(
	ctx context.Context,
	queryID uint64,
	txHeight uint64,
	block *neutrontypes.Block,
	submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo,
) error {
	proofStart := time.Now()
	hash := hex.EncodeToString(tmtypes.Tx(block.Tx.Data).Hash())
	neutronTxHash, err := r.submitter.SubmitTxProof(ctx, queryID, block)
	processedTx := relay.Transaction{
		Tx:     block.Tx,
		Height: txHeight,
	}
	if err != nil {
		return r.processFailedTxSubmission(err, queryID, hash, neutronTxHash, processedTx, proofStart)
	}
	return r.processSuccessfulTxSubmission(ctx, queryID, hash, neutronTxHash, processedTx, proofStart, submittedTxsTasksQueue)
}

// processSuccessfulTxSubmission stores the tx status in the storage and submits the PendingSubmittedTxInfo to
// submittedTxsTasksQueue.
func (r *TXProcessor) processSuccessfulTxSubmission(
	ctx context.Context,
	queryID uint64,
	hash string,
	neutronTxHash string,
	tx relay.Transaction,
	proofStart time.Time,
	submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo,
) error {
	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())
	err := r.storage.SetTxStatus(queryID, hash, neutronTxHash, relay.SubmittedTxInfo{
		Status: relay.Submitted,
	}, &tx)
	if err != nil {
		return fmt.Errorf("failed to store tx submit status: %w", err)
	}

	go r.delayedTxStatusCheck(ctx, relay.PendingSubmittedTxInfo{
		QueryID:         queryID,
		SubmittedTxHash: hash,
		NeutronHash:     neutronTxHash,
	}, submittedTxsTasksQueue)

	r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))

	return nil
}

// processFailedTxSubmission checks whether the error is ignored. If it's ignored, it stores the tx status in the
// storage; otherwise it escalates the error.
func (r *TXProcessor) processFailedTxSubmission(
	err error,
	queryID uint64,
	hash string,
	neutronTxHash string,
	tx relay.Transaction,
	proofStart time.Time,
) error {
	neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())
	r.logger.Error("could not submit proof", zap.Error(err), zap.Uint64("query_id", queryID))

	// check error with regexp
	if !r.ignoreErrorsRegexp.MatchString(err.Error()) {
		return relay.NewErrSubmitTxProofCritical(err)
	}

	errSetStatus := r.storage.SetTxStatus(
		queryID, hash, neutronTxHash, relay.SubmittedTxInfo{Status: relay.ErrorOnSubmit, Message: err.Error()}, &tx)
	if errSetStatus != nil {
		return fmt.Errorf("failed to store tx submit status: %w", errSetStatus)
	}

	return nil
}

// We submit the PendingSubmittedTxInfo only after checkSubmittedTxStatusDelay to reduce the possibility of
// unsuccessful checks (the block is 100% not ready here yet).
func (r TXProcessor) delayedTxStatusCheck(ctx context.Context, tx relay.PendingSubmittedTxInfo, submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo,
) {
	var t = time.NewTimer(r.checkSubmittedTxStatusDelay)
	select {
	case <-t.C:
		submittedTxsTasksQueue <- relay.PendingSubmittedTxInfo{
			QueryID:         tx.QueryID,
			SubmittedTxHash: tx.SubmittedTxHash,
			NeutronHash:     tx.NeutronHash,
		}
	case <-ctx.Done():
		r.logger.Info("Cancelled PendingSubmittedTxInfo delayed checking",
			zap.Uint64("query_id", tx.QueryID),
			zap.String("submitted_tx_hash", tx.SubmittedTxHash))
	}
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

// prepareHeaders returns two Headers for height and height+1 packed into *codectypes.Any value
// We need two blocks in Neutron to verify both delivery of tx and inclusion in block:
// - We need to know block X (`header`) to verify inclusion of transaction for block X (inclusion proof)
// - We need to know block X+1 (`nextHeader`) to verify response of transaction for block X
// since LastResultsHash is root hash of all results from the txs from the previous block (delivery proof)
//
// Arguments:
// `height` - remote chain block height X = transaction with such block height
func (r TXProcessor) prepareHeaders(ctx context.Context, txStruct relay.Transaction) (
	packedHeader *codectypes.Any, packedNextHeader *codectypes.Any, err error) {

	packedHeader, err = r.getTrustedHeader(ctx, txStruct.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get header with trusted height: %w", err)
	}

	packedNextHeader, err = r.getTrustedHeader(ctx, txStruct.Height+1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get next header with trusted height: %w", err)
	}

	return
}

func (r TXProcessor) getTrustedHeader(ctx context.Context, height uint64) (
	packedHeader *codectypes.Any, err error) {

	header, err := r.trustedHeaderFetcher.Fetch(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get header with trusted height: %w", err)
	}

	packedHeader, err = clienttypes.PackHeader(header)
	if err != nil {
		return nil, fmt.Errorf("failed to pack header: %w", err)
	}

	return
}
