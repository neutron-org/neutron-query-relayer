package txsubmitchecker

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/avast/retry-go/v4"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
	"time"
)

const TxCheckerNumWorkers = 4 // XXX: maybe this value should be configurable by user?

var (
	retryAttempts = retry.Attempts(4)
	retryDelay    = retry.Delay(1 * time.Second)
	retryError    = retry.LastErrorOnly(false)
)

type TxSubmitChecker struct {
	inChan     <-chan relay.PendingSubmittedTxInfo
	queue      chan relay.PendingSubmittedTxInfo
	storage    relay.Storage
	rpcClient  rpcclient.Client
	logger     *zap.Logger
	checkDelay time.Duration
}

func NewTxSubmitChecker(
	inQueue <-chan relay.PendingSubmittedTxInfo,
	storage relay.Storage,
	rpcClient rpcclient.Client,
	logger *zap.Logger,
	checkSubmittedTxStatusDelay uint64,
) *TxSubmitChecker {
	return &TxSubmitChecker{
		inQueue,
		make(chan relay.PendingSubmittedTxInfo),
		storage,
		rpcClient,
		logger,
		time.Duration(checkSubmittedTxStatusDelay) * time.Second,
	}
}

func (tc *TxSubmitChecker) Run(ctx context.Context) {
	// we don't want to start jobs before we read all pending txs from database,
	// hence we block on this read operation right in the beginning
	pending, err := tc.storage.GetAllPendingTxs()
	if err != nil {
		tc.logger.Fatal("failed to read pending txs from storage", zap.Error(err))
	}
	// these goroutines will eventually submit all pending txs into queue
	for _, tx := range pending {
		go tc.queueTx(*tx)
	}

	for i := 0; i < TxCheckerNumWorkers; i++ {
		go tc.worker(ctx)
	}

	for tx := range tc.inChan {
		go tc.queueTx(tx)
	}
}

func (tc *TxSubmitChecker) queueTx(tx relay.PendingSubmittedTxInfo) {
	tc.logger.Info(fmt.Sprintf("tx submit checker: new job: %s", tx.NeutronHash))
	age := time.Since(tx.SubmitTime)
	if age < 0 {
		tc.logger.Warn(fmt.Sprintf(
			"tx %s has negative age of %d milliseconds (submitted at %s)",
			tx.NeutronHash,
			age.Milliseconds(),
			tx.SubmitTime.Format(time.RFC3339Nano),
		))
	}
	if age >= 0 && age < tc.checkDelay {
		time.Sleep(tc.checkDelay - age)
	}
	tc.queue <- tx
}

func (tc *TxSubmitChecker) worker(ctx context.Context) {
	for {
		select {
		case tx := <-tc.queue:
			neutronHash, err := hex.DecodeString(tx.NeutronHash)
			if err != nil {
				tc.logger.Error(
					fmt.Sprintf("Failed to decode neutron hash %s (this must never happen)", tx.NeutronHash),
					zap.Error(err),
				)
				continue
			}

			txResponse, err := tc.retryGetTxStatusWithTimeout(neutronHash, 10*time.Second)
			if err != nil {
				tc.logger.Warn(fmt.Sprintf("Failed to call rpc://Tx(%s)", tx.NeutronHash), zap.Error(err))
				continue
			}

			if txResponse.TxResult.Code == abci.CodeTypeOK {
				tc.updateTxStatus(&tx, relay.SubmittedTxInfo{
					Status: relay.Committed,
				})
			} else {
				tc.updateTxStatus(&tx, relay.SubmittedTxInfo{
					Status:  relay.ErrorOnCommit,
					Message: fmt.Sprintf("%d %s", txResponse.TxResult.Code, txResponse.TxResult.Log),
				})
			}
		case <-ctx.Done():
			tc.logger.Info("Received termination signal, gracefully shutting down...")
			return
		}
	}
}

func (tc *TxSubmitChecker) retryGetTxStatusWithTimeout(neutronHash []byte, timeout time.Duration) (*coretypes.ResultTx, error) {
	var result *coretypes.ResultTx

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := retry.Do(func() error {
		var err error
		result, err = tc.rpcClient.Tx(ctx, neutronHash, false)
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
		tc.logger.Error(fmt.Sprintf("failed to update %s status in storage", tx.NeutronHash), zap.Error(err))
	} else {
		tc.logger.Info(fmt.Sprintf("tx submit checker: set job %s status to %v", tx.NeutronHash, status.Status))
	}
}
