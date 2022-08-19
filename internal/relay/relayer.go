package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/syndtr/goleveldb/leveldb"

	neutronmetrics "github.com/neutron-org/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	tmclient "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
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
	cfg          config.CosmosQueryRelayerConfig
	proofer      Proofer
	submitter    Submitter
	registry     *registry.Registry
	targetChain  *relayer.Chain
	neutronChain *relayer.Chain
	logger       *zap.Logger
	storage      Storage
}

func NewRelayer(
	cfg config.CosmosQueryRelayerConfig,
	proofer Proofer,
	submitter Submitter,
	registry *registry.Registry,
	srcChain,
	dstChain *relayer.Chain,
	logger *zap.Logger,
	stor Storage,
) Relayer {
	// TODO: after storage implementation update this func
	return Relayer{
		cfg:          cfg,
		proofer:      proofer,
		submitter:    submitter,
		registry:     registry,
		targetChain:  srcChain,
		neutronChain: dstChain,
		logger:       logger,
		storage:      stor,
	}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) error {
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

func (r Relayer) tryExtractInterchainQueries(event coretypes.ResultEvent) ([]queryEventMessage, error) {
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

func (r Relayer) proofMessage(ctx context.Context, m queryEventMessage) error {
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
			return fmt.Errorf("could not process %s with query_id=%d: Tx queries not allowed by configuraion", m.messageType, m.queryId)
		}

		err := r.initializeQuery(m.queryId)
		if err != nil {
			return fmt.Errorf("could not initialize query: %s with params=%s query_id=%d: %w",
				m.messageType, m.transactionsFilter, m.queryId, err)
		}

		var params recipientTransactionsParams
		queryLastHeight, _, err := r.storage.GetLastUpdateBlock(m.queryId)
		if err != nil {
			return fmt.Errorf("failed to get query last height from storage: %w", err)
		}

		// add filter by tx.height (tx.height>n)
		params[TxHeight] = fmt.Sprintf("%d", queryLastHeight)
		err = json.Unmarshal([]byte(m.transactionsFilter), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal transactions filter for %s with params=%s query_id=%d: %w",
				m.messageType, m.transactionsFilter, m.queryId, err)
		}

		blocks, keys, err := r.proofer.SearchTransactions(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}

		if len(blocks) == 0 {
			return nil
		}

		consensusStates, err := r.getConsensusStates(ctx)
		if err != nil {
			return fmt.Errorf("failed to get consensus states: %w", err)
		}

		for _, height := range keys {
			for _, tx := range blocks[height] {
				hash := string(tmtypes.Tx(tx.Data).Hash())
				txExists, err := r.storage.IsTxExists(m.queryId, hash)
				if err != nil {
					return fmt.Errorf("failed to check if transaction already exists: %w", err)
				}

				if txExists {
					r.logger.Debug("transaction already submitted", zap.Uint64("query_id", m.queryId), zap.String("hash", hash))
					continue
				}

				header, err := r.getHeaderWithBestTrustedHeight(ctx, consensusStates, height)
				if err != nil {
					return fmt.Errorf("failed to get header for src chain: %w", err)
				}

				packedHeader, err := clienttypes.PackHeader(header)
				if err != nil {
					return fmt.Errorf("failed to pack header: %w", err)
				}

				nextHeader, err := r.getHeaderWithBestTrustedHeight(ctx, consensusStates, height+1)
				if err != nil {
					return fmt.Errorf("failed to get next header for src chain: %w", err)
				}

				packedNextHeader, err := clienttypes.PackHeader(nextHeader)
				if err != nil {
					return fmt.Errorf("failed to pack header: %w", err)
				}

				proofStart := time.Now()
				if err := r.submitter.SubmitTxProof(ctx, m.queryId, r.neutronChain.PathEnd.ClientID, &neutrontypes.Block{
					Header:          packedHeader,
					NextBlockHeader: packedNextHeader,
					Tx:              tx,
				}); err != nil {
					neutronmetrics.IncFailedProofs()
					neutronmetrics.AddFailedProof(string(m.messageType), time.Since(proofStart).Seconds())

					err = r.storage.SetTxStatus(m.queryId, hash, err.Error(), uint64(latestHeight))
					if err != nil {
						return fmt.Errorf("failed to store tx: %w", err)
					}
					return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
				}

				neutronmetrics.IncSuccessProofs()
				neutronmetrics.AddSuccessProof(string(m.messageType), time.Since(proofStart).Seconds())

				err = r.storage.SetTxStatus(m.queryId, hash, Success, uint64(latestHeight))
				if err != nil {
					return fmt.Errorf("failed to store tx: %w", err)
				}

				r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", m.queryId))
			}
			err = r.storage.SetLastUpdateBlock(m.queryId, height)
			if err != nil {
				return fmt.Errorf("failed to save last height of query: %w", err)
			}

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

	st := time.Now()
	if err = r.submitter.SubmitProof(
		ctx,
		uint64(height-1),
		srcHeader.GetHeight().GetRevisionNumber(),
		queryID,
		r.cfg.NeutronChain.AllowKVCallbacks,
		proof,
		updateClientMsg,
	); err != nil {
		neutronmetrics.IncFailedProofs()
		neutronmetrics.AddFailedProof(messageType, time.Since(st).Seconds())
		return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", messageType, queryID, err)
	}
	neutronmetrics.IncSuccessProofs()
	neutronmetrics.AddSuccessProof(messageType, time.Since(st).Seconds())
	r.logger.Info("proof for query_id submitted successfully", zap.Uint64("query_id", queryID))
	return nil
}

// getConsensusStates returns light client consensus states from Neutron chain
func (r *Relayer) getConsensusStates(ctx context.Context) ([]clienttypes.ConsensusStateWithHeight, error) {
	// Without this hack it doesn't want to work with NewQueryClient
	provConcreteNeutronChain, ok := r.neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}

	qc := clienttypes.NewQueryClient(provConcreteNeutronChain)

	consensusStatesResponse, err := qc.ConsensusStates(ctx, &clienttypes.QueryConsensusStatesRequest{
		ClientId: r.neutronChain.ClientID(),
		Pagination: &query.PageRequest{
			// TODO: paging
			Limit:      math.MaxUint64,
			Reverse:    true,
			CountTotal: true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get consensus states for client ID %s: %w", r.neutronChain.ClientID(), err)
	}

	return consensusStatesResponse.ConsensusStates, nil
}

// getHeaderWithBestTrustedHeight returns an IBC Update Header which can be used to update an on chain
// light client on the Neutron chain.
//
// It has the same purpose as r.targetChain.ChainProvider.GetIBCUpdateHeader() but the difference is
// that getHeaderWithBestTrustedHeight() trys to find the best TrustedHeight for the header
// relying on existing light client's consensus states on the Neutron chain.
//
// The best trusted height for the height in this case is the closest one to some existed consensus state's height but not less
func (r *Relayer) getHeaderWithBestTrustedHeight(ctx context.Context, consensusStates []clienttypes.ConsensusStateWithHeight, height uint64) (ibcexported.Header, error) {
	start := time.Now()
	bestTrustedHeight := clienttypes.Height{
		RevisionNumber: 0,
		RevisionHeight: 0,
	}

	// TODO: since we should implement paging for getting the consensus states, maybe it's better to move searching of
	// 	the best height there
	for _, cs := range consensusStates {
		if height >= cs.Height.RevisionHeight && cs.Height.RevisionHeight > bestTrustedHeight.RevisionHeight {
			bestTrustedHeight = cs.Height
			// we won't find anything better
			if cs.Height.RevisionHeight == height {
				break
			}
		}
	}

	if bestTrustedHeight.IsZero() {
		return nil, fmt.Errorf("no satisfying trusted height found for height: %v", height)
	}

	// Without this hack we can't call InjectTrustedFields
	provConcreteTargetChain, ok := r.targetChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		neutronmetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("failed to cast ChainProvider to concrete type (cosmos.CosmosProvider)")
	}
	header, err := r.targetChain.ChainProvider.GetLightSignedHeaderAtHeight(ctx, int64(height))
	if err != nil {
		neutronmetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, err
	}

	tmHeader, ok := header.(*tmclient.Header)
	if !ok {
		neutronmetrics.AddFailedTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
		return nil, fmt.Errorf("trying to inject fields into non-tendermint headers")
	}

	tmHeader.TrustedHeight = bestTrustedHeight
	neutronmetrics.AddSuccessTargetChainGetter("GetLightSignedHeaderAtHeight", time.Since(start).Seconds())
	return provConcreteTargetChain.InjectTrustedFields(ctx, tmHeader, r.neutronChain.ChainProvider, r.neutronChain.PathEnd.ClientID)
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

	previous, ok, err := r.storage.GetLastUpdateBlock(queryID)
	if err != nil {
		return false, err
	}

	if !ok || previous+r.cfg.MinKvUpdatePeriod <= currentBlock {
		err := r.storage.SetLastUpdateBlock(queryID, currentBlock)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, fmt.Errorf("attempted to update query results too soon: last update was on block=%d, current block=%d, maximum update period=%d", previous, currentBlock, r.cfg.MinKvUpdatePeriod)
}

func (r *Relayer) CloseDb() error {
	err := r.storage.Close()
	if err != nil {
		return fmt.Errorf("couldn't close relayer's storage: %w", err)
	}

	return nil
}

// returns no err if query exists in storage, also initializes query with block = 0  if not exists yet
func (r *Relayer) initializeQuery(queryID uint64) error {
	_, _, err := r.storage.GetLastUpdateBlock(queryID)
	if err == leveldb.ErrNotFound {
		err = r.storage.SetLastUpdateBlock(queryID, 0)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check query: %w", err)
	}
	return nil
}
