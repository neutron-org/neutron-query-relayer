package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/syndtr/goleveldb/leveldb"
	"go.uber.org/zap"

	tmtypes "github.com/tendermint/tendermint/types"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// TxHeight describes tendermint filter by tx.height that we use to get only actual txs
const TxHeight = "tx.height"

// Relayer is controller for the whole app:
// 1. takes events from Neutron chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the Neutron chain
type Relayer struct {
	cfg                  config.NeutronQueryRelayerConfig
	proofer              Proofer
	submitter            Submitter
	targetChain          *relayer.Chain
	neutronChain         *relayer.Chain
	trustedHeaderFetcher TrustedHeaderFetcher
	subscriber           Subscriber
	logger               *zap.Logger
	storage              Storage
}

func NewRelayer(
	cfg config.NeutronQueryRelayerConfig,
	proofer Proofer,
	submitter Submitter,
	srcChain *relayer.Chain,
	dstChain *relayer.Chain,
	trustedHeaderFetcher TrustedHeaderFetcher,
	subscriber Subscriber,
	logger *zap.Logger,
	store Storage,
) *Relayer {
	return &Relayer{
		cfg:                  cfg,
		proofer:              proofer,
		submitter:            submitter,
		targetChain:          srcChain,
		neutronChain:         dstChain,
		trustedHeaderFetcher: trustedHeaderFetcher,
		subscriber:           subscriber,
		logger:               logger,
		storage:              store,
	}
}

// Run starts the relaying process: subscribes on the incoming interchain query messages from the
// Neutron and performs the queries by interacting with the target chain and submitting them to
// the Neutron chain.
func (r *Relayer) Run(ctx context.Context) error {
	r.logger.Info("subscribing to neutron chain events...")
	kvChan, txChan, err := r.subscriber.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to neutron events: %w", err)
	}
	r.logger.Info("successfully subscribed to neutron chain events")

	for {
		var (
			start     time.Time
			queryType neutrontypes.InterchainQueryType
			queryID   uint64
			err       error
		)
		select {
		case msg := <-kvChan:
			start = time.Now()
			queryType = neutrontypes.InterchainQueryTypeKV
			queryID = msg.QueryId
			err = r.processMessageKV(context.Background(), msg)
		case msg := <-txChan:
			start = time.Now()
			queryType = neutrontypes.InterchainQueryTypeTX
			queryID = msg.QueryId
			err = r.processMessageTX(context.Background(), msg)
		case <-ctx.Done():
			return r.stop()
		}
		if err != nil {
			r.logger.Error("could not process message", zap.Uint64("query_id", queryID), zap.Error(err))
			neutronmetrics.IncFailedRequests()
			neutronmetrics.AddFailedRequest(string(queryType), time.Since(start).Seconds())
		} else {
			neutronmetrics.IncSuccessRequests()
			neutronmetrics.AddSuccessRequest(string(queryType), time.Since(start).Seconds())
		}
	}
}

// stop finishes execution of relayer's auxiliary entities.
func (r *Relayer) stop() error {
	var failed bool
	if err := r.storage.Close(); err != nil {
		r.logger.Error("failed to close relayer's storage", zap.Error(err))
		failed = true
	} else {
		r.logger.Info("relayer's storage has been closed")
	}

	if err := r.subscriber.Unsubscribe(); err != nil {
		r.logger.Error("failed to unsubscribe", zap.Error(err))
		failed = true
	} else {
		r.logger.Info("subscriber has been stopped")
	}

	if failed {
		return fmt.Errorf("error occurred while stopping relayer, see recent logs for more info")
	}
	return nil
}

// processMessageKV handles an incoming KV interchain query message. It checks whether it's time
// to execute the query (based on the relayer's settings), queries values and proofs for the query
// keys, and submits the result to the Neutron chain.
func (r *Relayer) processMessageKV(ctx context.Context, m *MessageKV) error {
	r.logger.Debug("running proofMessageKV for msg", zap.Uint64("query_id", m.QueryId))
	latestHeight, err := r.targetChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get header for src chain: %w", err)
	}

	ok, err := r.isQueryOnTime(m.QueryId, uint64(latestHeight))
	if err != nil || !ok {
		return fmt.Errorf("error on checking previous query update with query_id=%d: %w", m.QueryId, err)
	}

	proofs, height, err := r.proofer.GetStorageValues(ctx, uint64(latestHeight), m.KVKeys)
	if err != nil {
		return fmt.Errorf("failed to get storage values with proofs for query_id=%d: %w", m.QueryId, err)
	}
	return r.submitProof(ctx, int64(height), m.QueryId, proofs)
}

// processMessageTX handles an incoming TX interchain query message. It fetches proven transactions
// from the target chain using the message transactions filter value, and submits the result to the
// Neutron chain.
func (r *Relayer) processMessageTX(ctx context.Context, m *MessageTX) error {
	r.logger.Debug("running proofMessageTX for msg", zap.Uint64("query_id", m.QueryId))
	latestHeight, err := r.targetChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to QueryLatestHeight: %w", err)
	}

	queryLastHeight, err := r.getLastQueryHeight(m.QueryId)
	if err != nil {
		return fmt.Errorf("could not get last query height: %w", err)
	}

	var params neutrontypes.TransactionsFilter
	if err = json.Unmarshal([]byte(m.TransactionsFilter), &params); err != nil {
		return fmt.Errorf("could not unmarshal transactions filter: %w", err)
	}
	// add filter by tx.height (tx.height>n)
	params = append(params, neutrontypes.TransactionsFilterItem{Field: TxHeight, Op: "gt", Value: queryLastHeight})
	txs, err := r.proofer.SearchTransactions(ctx, params)
	if err != nil {
		return fmt.Errorf("search for transactions failed: %w", err)
	}

	if len(txs) == 0 {
		return nil
	}

	// always process first searched tx due it could be the last tx in its block
	lastProcessedHeight := txs[0].Height
	for _, tx := range txs {
		// we don't update last query height until full block is processed
		// e.g. last query height = 0 and there are 3 txs in block 100 + 2 txs in block 101.
		// so until all 3 txs from block 100 has been proofed & sent, last query height will remain 0
		// and only starting from block 101 last query height will be set to 100
		if tx.Height > lastProcessedHeight {
			err = r.storage.SetLastQueryHeight(m.QueryId, lastProcessedHeight)
			if err != nil {
				return fmt.Errorf("failed to save last height of query: %w", err)
			}
		}
		lastProcessedHeight = tx.Height

		hash := string(tmtypes.Tx(tx.Tx.Data).Hash())
		txExists, err := r.storage.TxExists(m.QueryId, hash)
		if err != nil {
			return fmt.Errorf("failed to check if transaction already exists: %w", err)
		}

		if txExists {
			r.logger.Debug("transaction already submitted", zap.Uint64("query_id", m.QueryId), zap.String("hash", hash))
			continue
		}

		header, nextHeader, err := r.trustedHeaderFetcher.Fetch(ctx, tx.Height)
		if err != nil {
			// probably encountered one of two cases:
			// - tried to get headers for a transaction that is too old, since we could not find any consensus states for it
			// - tried to get headers for a transaction that is too new, and there is no light headers yet (unlikely, since we have retry in it)
			// this should not be a reason to stop submitting proofs for other transactions
			// NOTE: this can be bad as we can accidentally skip such transactions when new transactions will be processed and saved in storage
			r.logger.Info("could not get headers with trusted height for tx", zap.Error(err), zap.Uint64("query_id", m.QueryId), zap.String("hash", hash), zap.Uint64("height", tx.Height))
			continue
		}

		proofStart := time.Now()
		err = r.submitter.SubmitTxProof(ctx, m.QueryId, &neutrontypes.Block{
			Header:          header,
			NextBlockHeader: nextHeader,
			Tx:              tx.Tx,
		})
		if err != nil {
			neutronmetrics.IncFailedProofs()
			neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())

			if err := r.storage.SetTxStatus(m.QueryId, hash, err.Error()); err != nil {
				return fmt.Errorf("failed to store tx submission error: %w", err)
			}
			return fmt.Errorf("could not submit proof: %w", err)
		}
		neutronmetrics.IncSuccessProofs()
		neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeTX), time.Since(proofStart).Seconds())

		if err := r.storage.SetTxStatus(m.QueryId, hash, Success); err != nil {
			return fmt.Errorf("failed to mark tx as processed: %w", err)
		}
		r.logger.Info("proof for tx submitted successfully", zap.Uint64("query_id", m.QueryId))
	}
	err = r.storage.SetLastQueryHeight(m.QueryId, max(lastProcessedHeight, uint64(latestHeight)))
	if err != nil {
		return fmt.Errorf("failed to save last query height: %w", err)
	}
	return nil
}

// submitProof submits the proof for the given query on the given height and tracks the result.
func (r *Relayer) submitProof(
	ctx context.Context,
	height int64,
	queryID uint64,
	proof []*neutrontypes.StorageValue,
) error {
	srcHeader, err := r.getSrcChainHeader(ctx, height)
	if err != nil {
		return fmt.Errorf("failed to get header for height: %d: %w", height, err)
	}

	updateClientMsg, err := r.getUpdateClientMsg(ctx, srcHeader)
	if err != nil {
		return fmt.Errorf("failed to getUpdateClientMsg: %w", err)
	}

	start := time.Now()
	if err = r.submitter.SubmitProof(
		ctx,
		uint64(height-1),
		srcHeader.GetHeight().GetRevisionNumber(),
		queryID,
		r.cfg.AllowKVCallbacks,
		proof,
		updateClientMsg,
	); err != nil {
		neutronmetrics.IncFailedProofs()
		neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(start).Seconds())
		return fmt.Errorf("could not submit proof: %w", err)
	}
	neutronmetrics.IncSuccessProofs()
	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(start).Seconds())
	r.logger.Info("proof for kv query submitted successfully", zap.Uint64("query_id", queryID))
	return nil
}

func (r *Relayer) getSrcChainHeader(ctx context.Context, height int64) (ibcexported.Header, error) {
	start := time.Now()
	var srcHeader ibcexported.Header
	if err := retry.Do(func() error {
		var err error
		srcHeader, err = r.targetChain.ChainProvider.GetIBCUpdateHeader(ctx, height, r.neutronChain.ChainProvider, r.neutronChain.PathEnd.ClientID)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		r.logger.Info(
			"failed to GetIBCUpdateHeader", zap.Error(err))
	})); err != nil {
		return nil, err
	}
	neutronmetrics.RecordActionDuration("GetIBCUpdateHeader", time.Since(start).Seconds())
	return srcHeader, nil
}

func (r *Relayer) getUpdateClientMsg(ctx context.Context, srcHeader ibcexported.Header) (sdk.Msg, error) {
	start := time.Now()

	// Construct UpdateClient msg
	var updateMsgRelayer provider.RelayerMessage
	if err := retry.Do(func() error {
		var err error
		updateMsgRelayer, err = r.neutronChain.ChainProvider.UpdateClient(r.neutronChain.PathEnd.ClientID, srcHeader)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		r.logger.Error(
			"failed to build message", zap.Error(err))
	})); err != nil {
		return nil, err
	}

	updateMsgUnpacked, ok := updateMsgRelayer.(cosmos.CosmosMessage)
	if !ok {
		return nil, errors.New("failed to cast provider.RelayerMessage to cosmos.CosmosMessage")
	}
	neutronmetrics.RecordActionDuration("GetUpdateClientMsg", time.Since(start).Seconds())
	return updateMsgUnpacked.Msg, nil
}

// isQueryOnTime checks if query satisfies update period condition which is set by RELAYER_KV_UPDATE_PERIOD env, also modifies storage w last block
func (r *Relayer) isQueryOnTime(queryID uint64, currentBlock uint64) (bool, error) {
	// if it wasn't set in config
	if r.cfg.MinKvUpdatePeriod == 0 {
		return true, nil
	}

	previous, ok, err := r.storage.GetLastQueryHeight(queryID)
	if err != nil {
		return false, err
	}

	if !ok || previous+r.cfg.MinKvUpdatePeriod <= currentBlock {
		err := r.storage.SetLastQueryHeight(queryID, currentBlock)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, fmt.Errorf("attempted to update query results too soon: last update was on block=%d, current block=%d, maximum update period=%d", previous, currentBlock, r.cfg.MinKvUpdatePeriod)
}

// getLastQueryHeight returns last query height & no err if query exists in storage, also initializes query with height = 0  if not exists yet
func (r *Relayer) getLastQueryHeight(queryID uint64) (uint64, error) {
	height, _, err := r.storage.GetLastQueryHeight(queryID)
	if err == leveldb.ErrNotFound {
		err = r.storage.SetLastQueryHeight(queryID, 0)
		if err != nil {
			return 0, fmt.Errorf("failed to set a 0 last height for an unitilialised query: %w", err)
		}
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to check query in storage: %w", err)
	}
	return height, nil
}

func max(x, y uint64) uint64 {
	if x < y {
		return y
	}
	return x
}
