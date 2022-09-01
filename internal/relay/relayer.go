package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/syndtr/goleveldb/leveldb"

	neutronmetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	"github.com/neutron-org/cosmos-query-relayer/internal/registry"
	tmtypes "github.com/tendermint/tendermint/types"
)

// TxHeight describes tendermint filter by tx.height that we use to get only actual txs
const TxHeight = "tx.height"

// Relayer is controller for the whole app:
// 1. takes events from Neutron chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the Neutron chain
type Relayer struct {
	cfg              config.CosmosQueryRelayerConfig
	proofer          Proofer
	submitter        Submitter
	registry         *registry.Registry
	targetChain      *relayer.Chain
	neutronChain     *relayer.Chain
	consensusManager ConsensusManager
	logger           *zap.Logger
	storage          Storage
}

func NewRelayer(
	cfg config.CosmosQueryRelayerConfig,
	proofer Proofer,
	submitter Submitter,
	registry *registry.Registry,
	srcChain,
	dstChain *relayer.Chain,
	consensusManager ConsensusManager,
	logger *zap.Logger,
	store Storage,
) Relayer {
	return Relayer{
		cfg:              cfg,
		proofer:          proofer,
		submitter:        submitter,
		registry:         registry,
		targetChain:      srcChain,
		neutronChain:     dstChain,
		consensusManager: consensusManager,
		logger:           logger,
		storage:          store,
	}
}

func (r *Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) error {
	messages, err := r.tryExtractInterchainQueries(event)
	if err != nil {
		return fmt.Errorf("could not filter interchain query messages: %w", err)
	}
	if len(messages) == 0 {
		r.logger.Info("event has been skipped: it's not intended for us", zap.String("query", event.Query))
		return nil
	}

	for _, m := range messages {
		start := time.Now()
		if err := r.proofMessage(ctx, m); err != nil {
			r.logger.Error("could not process message", zap.Uint64("query_id", m.queryId), zap.Error(err))
			neutronmetrics.IncFailedRequests()
			neutronmetrics.AddFailedRequest(string(m.messageType), time.Since(start).Seconds())
		} else {
			neutronmetrics.IncSuccessRequests()
			neutronmetrics.AddSuccessRequest(string(m.messageType), time.Since(start).Seconds())
		}
	}

	// TODO: return a detailed error message here, e.g.
	// failed to prove N of M messages: latest error: %v (just an example, could be better)
	return err
}

func (r *Relayer) tryExtractInterchainQueries(event coretypes.ResultEvent) ([]queryEventMessage, error) {
	events := event.Events
	if len(events[zoneIdAttr]) == 0 {
		return nil, nil
	}

	if len(events[zoneIdAttr]) != len(events[kvKeyAttr]) ||
		len(events[zoneIdAttr]) != len(events[transactionsFilter]) ||
		len(events[zoneIdAttr]) != len(events[queryIdAttr]) ||
		len(events[zoneIdAttr]) != len(events[typeAttr]) {
		return nil, fmt.Errorf("cannot filter interchain query messages because events attributes length does not match for events=%v", events)
	}

	messages := make([]queryEventMessage, 0, len(events[zoneIdAttr]))

	for idx, zoneId := range events[zoneIdAttr] {
		if !(r.isTargetZone(zoneId) && r.isWatchedAddress(events[ownerAttr][idx])) {
			continue
		}

		queryIdStr := events[queryIdAttr][idx]
		queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
		if err != nil {
			r.logger.Info("invalid query_id format (not an uint)", zap.Error(err))
			continue
		}

		var (
			kvKeys                  neutrontypes.KVKeys
			transactionsFilterValue string
		)

		messageType := neutrontypes.InterchainQueryType(events[typeAttr][idx])

		switch messageType {
		case neutrontypes.InterchainQueryTypeKV:
			kvKeys, err = neutrontypes.KVKeysFromString(events[kvKeyAttr][idx])
			if err != nil {
				r.logger.Info("invalid kv_key attr", zap.Error(err))
				continue
			}
		case neutrontypes.InterchainQueryTypeTX:
			transactionsFilterValue = events[transactionsFilter][idx]
		default:
			r.logger.Info("unknown query_type", zap.String("query_type", string(messageType)))
			continue
		}

		messages = append(messages, queryEventMessage{queryId: queryId, messageType: messageType, kvKeys: kvKeys, transactionsFilter: transactionsFilterValue})
	}

	return messages, nil
}

func (r *Relayer) proofMessage(ctx context.Context, m queryEventMessage) error {
	r.logger.Debug("proofMessage", zap.String("message_type", string(m.messageType)))

	// TODO:
	// 1. move message handling logic from switch section to dedicated methods
	// 2. move the QueryLatestHeight call to the methods which require it
	latestHeight, err := r.targetChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to QueryLatestHeight: %w", err)
	}

	switch m.messageType {
	case neutrontypes.InterchainQueryTypeKV:
		ok, err := r.isQueryOnTime(m.queryId, uint64(latestHeight))
		if err != nil || !ok {
			return fmt.Errorf("error on checking previous query update with query_id=%d: %w", m.queryId, err)
		}

		proofs, height, err := r.proofer.GetStorageValues(ctx, uint64(latestHeight), m.kvKeys)
		if err != nil {
			return fmt.Errorf("failed to get storage values with proofs for query_id=%d: %w", m.queryId, err)
		}

		return r.submitProof(ctx, int64(height), m.queryId, string(m.messageType), proofs)
	case neutrontypes.InterchainQueryTypeTX:
		if !r.cfg.AllowTxQueries {
			return fmt.Errorf("could not process %s with query_id=%d: Tx queries not allowed by configuration", m.messageType, m.queryId)
		}

		queryLastHeight, err := r.getLastQueryHeight(m.queryId)
		if err != nil {
			return fmt.Errorf("could not get last query height: %s with params=%s query_id=%d: %w",
				m.messageType, m.transactionsFilter, m.queryId, err)
		}

		var params neutrontypes.TransactionsFilter
		err = json.Unmarshal([]byte(m.transactionsFilter), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal transactions filter for %s with params=%s query_id=%d: %w",
				m.messageType, m.transactionsFilter, m.queryId, err)
		}

		// add filter by tx.height (tx.height>n)
		params = append(params, neutrontypes.TransactionsFilterItem{Field: TxHeight, Op: "gt", Value: queryLastHeight})
		// TODO: not search for old transactions we cannot prove? (not within trusted period)
		txs, err := r.proofer.SearchTransactions(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}

		if len(txs) == 0 {
			return nil
		}

		// always process first searched tx due it could be the last tx in its block
		lastProcessedHeight := txs[0].Height
		for _, txItem := range txs {
			// we don't update last query height until full block is processed
			// e.g. last query height = 0 and there are 3 txs in block 100 + 2 txs in block 101.
			// so until all 3 txs from block 100 has been proofed & sent, last query height will remain 0
			// and only starting from block 101 last query height will be set to 100
			if txItem.Height > lastProcessedHeight {
				err = r.storage.SetLastQueryHeight(m.queryId, lastProcessedHeight)
				if err != nil {
					return fmt.Errorf("failed to save last height of query: %w", err)
				}
			}
			lastProcessedHeight = txItem.Height

			hash := string(tmtypes.Tx(txItem.Tx.Data).Hash())
			txExists, err := r.storage.TxExists(m.queryId, hash)
			if err != nil {
				return fmt.Errorf("failed to check if transaction already exists: %w", err)
			}

			if txExists {
				r.logger.Debug("transaction already submitted", zap.Uint64("query_id", m.queryId), zap.String("hash", hash))
				continue
			}

			header, nextHeader, err := r.consensusManager.GetPackedHeadersWithTrustedHeight(ctx, txItem.Height)
			if err != nil {
				// probably tried to get headers for a transaction that is too old, since we could not find any
				// this should not be a reason to stop submitting proofs for other transactions
				r.logger.Info("could not get headers with trusted height for tx", zap.Error(err), zap.Uint64("query_id", m.queryId), zap.String("hash", hash), zap.Uint64("height", txItem.Height))
				continue
			}

			proofStart := time.Now()
			if err := r.submitter.SubmitTxProof(ctx, m.queryId, r.neutronChain.PathEnd.ClientID, &neutrontypes.Block{
				Header:          header,
				NextBlockHeader: nextHeader,
				Tx:              txItem.Tx,
			}); err != nil {
				neutronmetrics.IncFailedProofs()
				neutronmetrics.AddFailedProof(string(m.messageType), time.Since(proofStart).Seconds())

				setErr := r.storage.SetTxStatus(m.queryId, hash, err.Error())
				if setErr != nil {
					return fmt.Errorf("failed to store tx: %w", setErr)
				}
				return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
			}

			neutronmetrics.IncSuccessProofs()
			neutronmetrics.AddSuccessProof(string(m.messageType), time.Since(proofStart).Seconds())

			err = r.storage.SetTxStatus(m.queryId, hash, Success)
			if err != nil {
				return fmt.Errorf("failed to store tx: %w", err)
			}

			r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", m.queryId))

		}
		err = r.storage.SetLastQueryHeight(m.queryId, max(lastProcessedHeight, uint64(latestHeight)))
		if err != nil {
			return fmt.Errorf("failed to save last height of query: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unknown query messageType=%s", m.messageType)
	}

}

// submitProof submits the proof for the given query on the given height and tracks the result.
func (r *Relayer) submitProof(
	ctx context.Context,
	height int64,
	queryID uint64,
	messageType string,
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
		neutronmetrics.AddFailedProof(messageType, time.Since(start).Seconds())
		return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", messageType, queryID, err)
	}
	neutronmetrics.IncSuccessProofs()
	neutronmetrics.AddSuccessProof(messageType, time.Since(start).Seconds())
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

// isTargetZone returns true if the zoneID is the relayer's target zone id.
func (r *Relayer) isTargetZone(zoneID string) bool {
	return r.targetChain.ChainID() == zoneID
}

// isWatchedAddress returns true if the address is within the registry watched addresses or there
// are no registry watched addresses configured for the Relayer meaning all addresses are watched.
func (r *Relayer) isWatchedAddress(address string) bool {
	return r.registry.IsEmpty() || r.registry.Contains(address)
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

func (r *Relayer) CloseStorage() error {
	err := r.storage.Close()
	if err != nil {
		return fmt.Errorf("couldn't close relayer's storage: %w", err)
	}

	return nil
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
