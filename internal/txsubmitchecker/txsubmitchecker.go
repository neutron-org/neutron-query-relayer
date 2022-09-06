package txsubmitchecker

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"go.uber.org/zap"
	"time"
)

const TxCheckThreads = 4 // XXX: maybe this value should be configurable by user?

type TxSubmitChecker struct {
	inChan     <-chan relay.SubmittedTxInfo
	queue      chan relay.SubmittedTxInfo
	storage    relay.Storage
	rpcClient  rpcclient.Client
	logger     *zap.Logger
	checkDelay uint64
}

func NewTxSubmitChecker(
	inQueue <-chan relay.SubmittedTxInfo,
	storage relay.Storage,
	rpcClient rpcclient.Client,
	logger *zap.Logger,
	checkSubmittedTxStatusDelay uint64,
) TxSubmitChecker {
	return TxSubmitChecker{
		inQueue,
		make(chan relay.SubmittedTxInfo),
		storage,
		rpcClient,
		logger,
		checkSubmittedTxStatusDelay,
	}
}

func (tc *TxSubmitChecker) Run() {
	// we don't want to start jobs before we read all pending txs from database,
	// hence we block on this read right in the beginning
	pending, err := tc.storage.GetAllPendingTxs()
	if err != nil {
		tc.logger.Fatal("failed to read pending txs from storage", zap.Error(err))
	}
	// these goroutines will eventually submit all pending txs into queue
	for _, tx := range pending {
		go tc.queueTx(*tx)
	}

	for i := 0; i < TxCheckThreads; i++ {
		go tc.workerThread()
	}

	for tx := range tc.inChan {
		go tc.queueTx(tx)
	}
}

func (tc *TxSubmitChecker) queueTx(tx relay.SubmittedTxInfo) {
	tc.logger.Info(fmt.Sprintf("tx submit checker: new job: %s", tx.NeutronHash))
	age := time.Now().Sub(tx.SubmitTime).Milliseconds()
	if age < 0 {
		tc.logger.Warn(fmt.Sprintf(
			"tx %s has negative age of %d milliseconds (submitted at %s)",
			tx.NeutronHash,
			age,
			tx.SubmitTime.Format(time.RFC3339Nano),
		))
	}
	if age >= 0 && age < int64(tc.checkDelay) {
		time.Sleep(time.Duration(int64(tc.checkDelay)-age) * time.Millisecond)
	}
	tc.queue <- tx
}

func (tc *TxSubmitChecker) workerThread() {
	// TODO: this thread runs for infinite amount of time, there must be some means to gracefully stop it
	for tx := range tc.queue {
		neutronHash, err := hex.DecodeString(tx.NeutronHash)
		if err != nil {
			tc.logger.Error(
				fmt.Sprintf("Failed to decode neutron hash %s (this must never happen)", tx.NeutronHash),
				zap.Error(err),
			)
			continue
		}

		txResponse, err := tc.rpcClient.Tx(context.TODO(), neutronHash, false)
		if err != nil {
			tc.logger.Info(fmt.Sprintf("Failed to call rpc://Tx(%s)", tx.NeutronHash), zap.Error(err))
			// try again after `checkSubmittedTxStatusDelay` milliseconds
			go func(tx relay.SubmittedTxInfo) {
				time.Sleep(time.Duration(tc.checkDelay) * time.Millisecond)
				tc.queue <- tx
			}(tx)
			continue
		}

		if txResponse.TxResult.Code == 0 {
			tc.updateTxStatus(&tx, relay.Committed)
		} else {
			tc.updateTxStatus(&tx, relay.ErrorOnCommit)
		}
	}
}

func (tc *TxSubmitChecker) updateTxStatus(tx *relay.SubmittedTxInfo, status string) {
	err := tc.storage.SetTxStatus(tx.QueryID, tx.SubmittedTxHash, tx.NeutronHash, status)
	if err != nil {
		// XXX: I expect storage to work all the time, am I shooting myself in a foot?
		tc.logger.Fatal(fmt.Sprintf("failed to update %s status in storage", tx.NeutronHash), zap.Error(err))
	}
	tc.logger.Info(fmt.Sprintf("tx submit checker: set job %s status to %s", tx.NeutronHash, status))
}
