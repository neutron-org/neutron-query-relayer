package app

import (
	"context"
	"fmt"

	"github.com/neutron-org/neutron-query-relayer/internal/kvprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/txprocessor"

	"github.com/avast/retry-go/v4"
	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"

	"github.com/neutron-org/neutron-query-relayer/internal/storage"

	"time"

	rpcclienthttp "github.com/cometbft/cometbft/rpc/client/http"
	"go.uber.org/zap"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	relaysubscriber "github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	"github.com/neutron-org/neutron-query-relayer/internal/txsubmitchecker"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

var (
	Version = ""
	Commit  = ""
)

const (
	AppContext                   = "app"
	SubscriberContext            = "subscriber"
	RelayerContext               = "relayer"
	TargetChainRPCClientContext  = "target_chain_rpc"
	NeutronChainRPCClientContext = "neutron_chain_rpc"
	TargetChainProviderContext   = "target_chain_provider"
	NeutronChainProviderContext  = "neutron_chain_provider"
	TxSenderContext              = "tx_sender"
	TxProcessorContext           = "tx_processor"
	TxSubmitCheckerContext       = "tx_submit_checker"
	TrustedHeadersFetcherContext = "trusted_headers_fetcher"
	KVProcessorContext           = "kv_processor"
)

// retries configuration for fetching connection info
var (
	rtyAtt = retry.Attempts(uint(5))
	rtyDel = retry.Delay(time.Second * 10)
	rtyErr = retry.LastErrorOnly(true)
)

func NewDefaultSubscriber(cfg config.NeutronQueryRelayerConfig, logRegistry *nlogger.Registry) (relay.Subscriber, error) {
	watchedMsgTypes := []neutrontypes.InterchainQueryType{neutrontypes.InterchainQueryTypeKV}
	if cfg.AllowTxQueries {
		watchedMsgTypes = append(watchedMsgTypes, neutrontypes.InterchainQueryTypeTX)
	}

	subscriber, err := relaysubscriber.NewSubscriber(
		&subscriber.SubscriberConfig{
			RPCAddress:      cfg.NeutronChain.RPCAddr,
			RESTAddress:     cfg.NeutronChain.RESTAddr,
			Timeout:         cfg.NeutronChain.Timeout,
			ConnectionID:    cfg.NeutronChain.ConnectionID,
			WatchedTypes:    watchedMsgTypes,
			WatchedQueryIDs: cfg.Registry.QueryIDS,
			Registry:        registry.New(cfg.Registry),
		},
		logRegistry.Get(SubscriberContext),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a NewSubscriber: %s", err)
	}

	return subscriber, nil
}

func NewDefaultTxSubmitChecker(cfg config.NeutronQueryRelayerConfig, logRegistry *nlogger.Registry,
	storage relay.Storage) (relay.TxSubmitChecker, error) {
	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create NewRPCClient: %w", err)
	}

	return txsubmitchecker.NewTxSubmitChecker(
		storage,
		neutronClient,
		logRegistry.Get(TxSubmitCheckerContext),
	), nil
}

// NewDefaultRelayer returns a relayer built with cfg.
func NewDefaultRelayer(
	cfg config.NeutronQueryRelayerConfig,
	logRegistry *nlogger.Registry,
	storage relay.Storage,
	deps *DependencyContainer,
) (*relay.Relayer, error) {
	var (
		txProcessor = txprocessor.NewTxProcessor(
			deps.GetTrustedHeaderFetcher(), storage, deps.GetProofSubmitter(), logRegistry.Get(TxProcessorContext), cfg.CheckSubmittedTxStatusDelay, cfg.IgnoreErrorsRegex)
		kvProcessor = kvprocessor.NewKVProcessor(
			deps.GetTrustedHeaderFetcher(),
			deps.GetTargetQuerier(),
			cfg.MinKvUpdatePeriod,
			logRegistry.Get(KVProcessorContext),
			deps.GetProofSubmitter(),
			storage,
			deps.GetTargetChain(),
			deps.GetNeutronChain(),
		)
		relayer = relay.NewRelayer(
			cfg,
			deps.GetTxQuerier(),
			storage,
			txProcessor,
			kvProcessor,
			deps.GetTargetChain(),
			logRegistry.Get(RelayerContext),
		)
	)
	return relayer, nil
}

func NewDefaultStorage(cfg config.NeutronQueryRelayerConfig, logger *zap.Logger) (relay.Storage, error) {
	var (
		err            error
		leveldbStorage relay.Storage
	)

	leveldbStorage, err = storage.NewLevelDBStorage(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create NewLevelDBStorage: %w", err)
	}

	return leveldbStorage, nil
}

func loadChains(
	ctx context.Context,
	cfg config.NeutronQueryRelayerConfig,
	logRegistry *nlogger.Registry,
	connParams *connectionParams,
) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
	targetChain, err = relay.GetTargetChain(logRegistry.Get(TargetChainProviderContext), cfg.TargetChain, connParams.targetChainID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load target chain from env: %w", err)
	}

	if err := targetChain.AddPath(connParams.targetClientID, connParams.targetConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to source chain: %w", err)
	}

	if err := targetChain.ChainProvider.Init(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	neutronChain, err = relay.GetNeutronChain(logRegistry.Get(NeutronChainProviderContext), cfg.NeutronChain, connParams.neutronChainID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load neutron chain from env: %w", err)
	}

	if err := neutronChain.AddPath(connParams.neutronClientID, cfg.NeutronChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to destination chain: %w", err)
	}

	if err := neutronChain.ChainProvider.Init(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	return neutronChain, targetChain, nil
}

type connectionParams struct {
	neutronChainID  string
	neutronClientID string

	targetChainID      string
	targetClientID     string
	targetConnectionID string
}

func loadConnParams(ctx context.Context, neutronClient, targetClient *rpcclienthttp.HTTP, neutronRestAddress string, neutronConnectionId string, logger *zap.Logger) (*connectionParams, error) {
	restClient, err := raw.NewRESTClient(neutronRestAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get newRESTClient: %w", err)
	}

	targetStatus, err := targetClient.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target chain status: %w", err)
	}

	neutronStatus, err := neutronClient.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch neutron chain status: %w", err)
	}

	var queryResponse *query.IbcCoreConnectionV1ConnectionOK
	if err := retry.Do(func() error {
		var err error

		queryResponse, err = restClient.Query.IbcCoreConnectionV1Connection(&query.IbcCoreConnectionV1ConnectionParams{
			ConnectionID: neutronConnectionId,
			Context:      ctx,
		})
		if err != nil {
			return err
		}

		if queryResponse.GetPayload().Connection.Counterparty.ConnectionID == "" {
			return fmt.Errorf("empty target connection ID")
		}

		if queryResponse.GetPayload().Connection.Counterparty.ClientID == "" {
			return fmt.Errorf("empty target client ID")
		}

		return nil
	}, retry.Context(ctx), rtyAtt, rtyDel, rtyErr, retry.OnRetry(func(n uint, err error) {
		logger.Info(
			"failed to query ibc connection info", zap.Error(err))
	})); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query ibc connection info: %w", err)
	}

	connParams := connectionParams{
		neutronChainID:     neutronStatus.NodeInfo.Network,
		targetChainID:      targetStatus.NodeInfo.Network,
		neutronClientID:    queryResponse.GetPayload().Connection.ClientID,
		targetClientID:     queryResponse.GetPayload().Connection.Counterparty.ClientID,
		targetConnectionID: queryResponse.GetPayload().Connection.Counterparty.ConnectionID,
	}

	logger.Info("loaded conn params",
		zap.String("neutron_chain_id", connParams.neutronChainID),
		zap.String("target_chain_id", connParams.targetChainID),
		zap.String("neutron_client_id", connParams.neutronClientID),
		zap.String("target_client_id", connParams.targetClientID),
		zap.String("target_connection_id", connParams.targetConnectionID))

	return &connParams, nil
}
