package txsubmitchecker

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
)

var (
	retryAttempts  = retry.Attempts(4)
	retryDelay     = retry.Delay(1 * time.Second)
	retryError     = retry.LastErrorOnly(false)
	requestTimeout = 10 * time.Second
)

type TxSubmitChecker struct {
	storage   relay.Storage
	rpcClient rpcclient.Client
	logger    *zap.Logger
}

func NewTxSubmitChecker(
	storage relay.Storage,
	rpcClient rpcclient.Client,
	logger *zap.Logger,
) *TxSubmitChecker {
	return &TxSubmitChecker{
		storage:   storage,
		rpcClient: rpcClient,
		logger:    logger,
	}
}

func (tc *TxSubmitChecker) Run(ctx context.Context, submittedTxsTasksQueue <-chan relay.PendingSubmittedTxInfo) error {
	// Read and process all pending submitted transactions on startup.
	pending, err := tc.storage.GetAllPendingTxs()
	if err != nil {
		return fmt.Errorf("failed to read pending txs from storage: %w", err)
	}

	for _, tx := range pending {
		if err := tc.processSubmittedTx(ctx, tx); err != nil {
			tc.logger.Error("Failed to processSubmittedTx (on startup)",
				zap.Error(err), zap.String("tx_neutron_hash", tx.NeutronHash),
				zap.String("tx_submitted_hash", tx.SubmittedTxHash))
		}
	}

	for {
		select {
		case tx := <-submittedTxsTasksQueue:
			if err := tc.processSubmittedTx(ctx, &tx); err != nil {
				tc.logger.Error("Failed to processSubmittedTx",
					zap.Error(err), zap.String("tx_neutron_hash", tx.NeutronHash),
					zap.String("tx_submitted_hash", tx.SubmittedTxHash))
			}
		case <-ctx.Done():
			tc.logger.Info("Context cancelled, shutting down TxSubmitChecker...")
			return nil
		}
	}
}

func (tc *TxSubmitChecker) processSubmittedTx(ctx context.Context, tx *relay.PendingSubmittedTxInfo) error {
	neutronHash, err := hex.DecodeString(tx.NeutronHash)
	if err != nil {
		return fmt.Errorf("failed to DecodeString: %w", err)
	}

	txResponse, err := tc.retryGetTxStatus(ctx, neutronHash)
	if err != nil {
		return fmt.Errorf("failed to retryGetTxStatus: %w", err)
	}

	if txResponse.TxResult.Code == abci.CodeTypeOK {
		tc.updateTxStatus(tx, relay.SubmittedTxInfo{
			Status: relay.Committed,
		})
	} else {
		tc.updateTxStatus(tx, relay.SubmittedTxInfo{
			Status:  relay.ErrorOnCommit,
			Message: fmt.Sprintf("%d", txResponse.TxResult.Code),
		})
	}

	return nil
}

func (tc *TxSubmitChecker) retryGetTxStatus(
	ctx context.Context,
	neutronHash []byte,
) (*coretypes.ResultTx, error) {
	var result *coretypes.ResultTx
	if err := retry.Do(func() error {
		timeoutCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()
		var err error
		result, err = tc.rpcClient.Tx(timeoutCtx, neutronHash, false)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), retryAttempts, retryDelay, retryError); err != nil {
		return nil, err
	}

	return result, nil
}

func (tc *TxSubmitChecker) updateTxStatus(tx *relay.PendingSubmittedTxInfo, status relay.SubmittedTxInfo) {
	err := tc.storage.SetTxStatus(tx.QueryID, tx.SubmittedTxHash, tx.NeutronHash, status)
	if err != nil {
		tc.logger.Error(
			"failed to update tx status in storage",
			zap.String("neutron_hash", tx.NeutronHash),
			zap.Error(err),
		)
	} else {
		tc.logger.Info(
			"set tx status",
			zap.String("neutron_hash", tx.NeutronHash),
			zap.String("status", string(status.Status)),
		)
	}
}
