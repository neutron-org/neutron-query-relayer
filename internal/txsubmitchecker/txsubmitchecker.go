package txsubmitchecker

import (
	"context"
	"encoding/hex"
	"fmt"
	instrumenters "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"time"

	"github.com/avast/retry-go/v4"
	abci "github.com/tendermint/tendermint/abci/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"
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

func (tc *TxSubmitChecker) Run(ctx context.Context) error {
	// we don't want to start jobs before we read all pending txs from database,
	// hence we block on this read operation right in the beginning
	pending, err := tc.storage.GetAllPendingTxs()
	if err != nil {
		return fmt.Errorf("failed to read pending txs from storage: %w", err)
	}

	// these goroutines will eventually submit all pending txs into queue
	for _, tx := range pending {
		go tc.queueTx(*tx)
	}

	for i := 0; i < TxCheckerNumWorkers; i++ {
		go tc.worker(ctx)
	}

	go func() {
		for tx := range tc.inChan {
			go tc.queueTx(tx)
		}
	}()

	return nil
}

func (tc *TxSubmitChecker) queueTx(tx relay.PendingSubmittedTxInfo) {
	tc.logger.Info("new job", zap.String("neutron_hash", tx.NeutronHash))
	age := time.Since(tx.SubmitTime)
	if age < 0 {
		tc.logger.Warn(
			"tx has negative age",
			zap.String("neutron_hash", tx.NeutronHash),
			zap.Int64("age_nsecs", age.Nanoseconds()),
			zap.Time("submitted_at", tx.SubmitTime),
		)
	}
	if age >= 0 && age < tc.checkDelay {
		time.Sleep(tc.checkDelay - age)
	}
	tc.queue <- tx
}

func (tc *TxSubmitChecker) worker(ctx context.Context) {
	tc.logger.Info("init worker thread")

	for {
		select {
		case tx := <-tc.queue:
			neutronHash, err := hex.DecodeString(tx.NeutronHash)
			if err != nil {
				tc.logger.Error(
					"failed to decode hash",
					zap.String("neutron_hash", tx.NeutronHash),
					zap.Error(err),
				)
				continue
			}

			txResponse, err := tc.retryGetTxStatusWithTimeout(ctx, neutronHash, 10*time.Second)
			if err != nil {
				tc.logger.Warn(
					"failed to get tx status from rpc",
					zap.String("neutron_hash", tx.NeutronHash),
					zap.Error(err),
				)
				continue
			}

			if txResponse.TxResult.Code == abci.CodeTypeOK {
				instrumenters.IncSuccessTxSubmit()
				tc.updateTxStatus(&tx, relay.SubmittedTxInfo{
					Status: relay.Committed,
				})
			} else {
				instrumenters.IncFailedTxSubmit()
				tc.updateTxStatus(&tx, relay.SubmittedTxInfo{
					Status:  relay.ErrorOnCommit,
					Message: fmt.Sprintf("%d", txResponse.TxResult.Code),
				})
			}
		case <-ctx.Done():
			tc.logger.Info("worker has been stopped by context")
			return
		}
	}
}

func (tc *TxSubmitChecker) retryGetTxStatusWithTimeout(
	ctx context.Context,
	neutronHash []byte,
	timeout time.Duration,
) (*coretypes.ResultTx, error) {
	var result *coretypes.ResultTx

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := retry.Do(func() error {
		var err error
		result, err = tc.rpcClient.Tx(timeoutCtx, neutronHash, false)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(timeoutCtx), retryAttempts, retryDelay, retryError); err != nil {
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
