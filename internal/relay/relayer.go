package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	neutronmetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/syndtr/goleveldb/leveldb"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	"github.com/syndtr/goleveldb/leveldb"
	"go.uber.org/zap"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

// TxHeight describes tendermint filter by tx.height that we use to get only actual txs
const TxHeight = "tx.height"

// Relayer is controller for the whole app:
// 1. takes events from Neutron chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the Neutron chain
type Relayer struct {
	cfg          config.NeutronQueryRelayerConfig
	proofer      Proofer
	txQuerier    TXQuerier
	submitter    Submitter
	targetChain  *relayer.Chain
	neutronChain *relayer.Chain
	subscriber   Subscriber
	logger       *zap.Logger
	storage      Storage
	txProcessor  TXProcessor
}

func NewRelayer(
	cfg config.NeutronQueryRelayerConfig,
	proofer Proofer,
	txQuerier TXQuerier,
	submitter Submitter,
	srcChain *relayer.Chain,
	dstChain *relayer.Chain,
	subscriber Subscriber,
	logger *zap.Logger,
	store Storage,
	txProcessor TXProcessor,
) *Relayer {
	return &Relayer{
		cfg:          cfg,
		txQuerier:    txQuerier,
		proofer:      proofer,
		submitter:    submitter,
		targetChain:  srcChain,
		neutronChain: dstChain,
		subscriber:   subscriber,
		logger:       logger,
		storage:      store,
		txProcessor:  txProcessor,
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
			neutronmetrics.AddFailedRequest(string(queryType), time.Since(start).Seconds())
		} else {
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
	r.logger.Debug("running processMessageKV for msg", zap.Uint64("query_id", m.QueryId))
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

func (r Relayer) buildTxQuery(m *MessageTX) (neutrontypes.TransactionsFilter, error) {
	queryLastHeight, err := r.getLastQueryHeight(m.QueryId)
	if err != nil {
		return nil, fmt.Errorf("could not get last query height: %w", err)
	}

	var params neutrontypes.TransactionsFilter
	if err = json.Unmarshal([]byte(m.TransactionsFilter), &params); err != nil {
		return nil, fmt.Errorf("could not unmarshal transactions filter: %w", err)
	}
	// add filter by tx.height (tx.height>n)
	params = append(params, neutrontypes.TransactionsFilterItem{Field: TxHeight, Op: "gt", Value: queryLastHeight})
	return params, nil
}

// processMessageTX handles an incoming TX interchain query message. It fetches proven transactions
// from the target chain using the message transactions filter value, and submits the result to the
// Neutron chain.
func (r *Relayer) processMessageTX(ctx context.Context, m *MessageTX) error {
	r.logger.Debug("running processMessageTX for msg", zap.Uint64("query_id", m.QueryId))
	queryParams, err := r.buildTxQuery(m)
	if err != nil {
		return fmt.Errorf("failed to build tx query params: %w", err)
	}

	txs := r.txQuerier.SearchTransactions(ctx, queryParams)
	if err != nil {
		return fmt.Errorf("search for transactions failed: %w", err)
	}

	lastProcessedHeight, err := r.txProcessor.ProcessAndSubmit(ctx, m.QueryId, txs)
	if err != nil {
		return fmt.Errorf("failed to process txs: %w", err)
	}
	if r.txQuerier.Err() != nil {
		return fmt.Errorf("failed to query txs: %w", r.txQuerier.Err())
	}
	err = r.storage.SetLastQueryHeight(m.QueryId, lastProcessedHeight)
	if err != nil {
		return fmt.Errorf("failed to save last height of query: %w", err)
	}
	return err
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

	st := time.Now()
	if err = r.submitter.SubmitKVProof(
		ctx,
		uint64(height-1),
		srcHeader.GetHeight().GetRevisionNumber(),
		queryID,
		r.cfg.AllowKVCallbacks,
		proof,
		updateClientMsg,
	); err != nil {
		neutronmetrics.AddFailedProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(st).Seconds())
		return fmt.Errorf("could not submit proof: %w", err)
	}
	neutronmetrics.AddSuccessProof(string(neutrontypes.InterchainQueryTypeKV), time.Since(st).Seconds())
	r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))
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
		neutronmetrics.AddFailedTargetChainGetter("GetIBCUpdateHeader", time.Since(start).Seconds())
		return nil, err
	}
	neutronmetrics.AddSuccessTargetChainGetter("GetIBCUpdateHeader", time.Since(start).Seconds())
	return srcHeader, nil
}

func (r *Relayer) getUpdateClientMsg(ctx context.Context, srcHeader ibcexported.Header) (sdk.Msg, error) {
	start := time.Now()
	// Query IBC Update Header

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
	neutronmetrics.AddSuccessTargetChainGetter("GetUpdateClientMsg", time.Since(start).Seconds())
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
