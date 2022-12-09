package app

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/avast/retry-go/v4"
	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"

	"time"

	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"
	"go.uber.org/zap"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	"github.com/neutron-org/neutron-query-relayer/internal/kvprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/relayer_keyring"
	"github.com/neutron-org/neutron-query-relayer/internal/storage"
	"github.com/neutron-org/neutron-query-relayer/internal/submit"
	relaysubscriber "github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	"github.com/neutron-org/neutron-query-relayer/internal/trusted_headers"
	"github.com/neutron-org/neutron-query-relayer/internal/txprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/txquerier"
	"github.com/neutron-org/neutron-query-relayer/internal/txsubmitchecker"
	neutronapp "github.com/neutron-org/neutron/app"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

var (
	Version string = ""
	Commit  string = ""
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
		cfg.NeutronChain.RPCAddr,
		cfg.NeutronChain.RESTAddr,
		cfg.NeutronChain.ConnectionID,
		registry.New(cfg.Registry),
		watchedMsgTypes,
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
	ctx context.Context,
	cfg config.NeutronQueryRelayerConfig,
	logRegistry *nlogger.Registry,
	storage relay.Storage) (*relay.Relayer, error) {
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()

	logger := logRegistry.Get(AppContext)

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddr, cfg.TargetChain.Timeout)
	if err != nil {
		return nil, fmt.Errorf("could not initialize target rpc client: %w", err)
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout)
	if err != nil {
		return nil, fmt.Errorf("cannot create neutron client: %w", err)
	}

	connParams, err := loadConnParams(ctx, neutronClient, targetClient, cfg.NeutronChain.RESTAddr,
		cfg.NeutronChain.ConnectionID, logRegistry.Get(AppContext))
	if err != nil {
		return nil, fmt.Errorf("cannot load network params: %w", err)
	}

	targetQuerier, err := tmquerier.NewQuerier(targetClient, connParams.targetChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to target chain: %w", err)
	}

	codec := raw.MakeCodecDefault()
	keybase, keyName, err := relayer_keyring.InitializeKeyring(
		cfg.NeutronChain.KeyringBackend,
		cfg.NeutronChain.KeyringPassword,
		cfg.NeutronChain.HomeDir,
		cfg.NeutronChain.SignKeyName,
		cfg.NeutronChain.SignKeySeed,
		cfg.NeutronChain.SignKeyHdPath,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize keybase: %w", err)
	}
	logger.Debug("keyring initialized", zap.String("keyring_backend", cfg.NeutronChain.KeyringBackend))

	txSender, err := submit.NewTxSender(ctx, neutronClient, codec.Marshaller, *cfg.NeutronChain, keybase, keyName, logRegistry.Get(TxSenderContext), connParams.neutronChainID)
	if err != nil {
		return nil, fmt.Errorf("cannot create tx sender: %w", err)
	}

	neutronChain, err := loadNeutronChain(cfg, keybase, keyName, logRegistry, connParams)
	if err != nil {
		return nil, fmt.Errorf("failed to load Neutron chain: %w", err)
	}

	targetChain, err := loadTargetChain(cfg, logRegistry, connParams)
	if err != nil {
		return nil, fmt.Errorf("failed to load target chain: %w", err)
	}

	var (
		proofSubmitter       = submit.NewSubmitterImpl(txSender, cfg.AllowKVCallbacks, neutronChain.PathEnd.ClientID)
		txQuerier            = txquerier.NewTXQuerySrv(targetQuerier.Client)
		trustedHeaderFetcher = trusted_headers.NewTrustedHeaderFetcher(neutronChain, targetChain, logRegistry.Get(TrustedHeadersFetcherContext))
		txProcessor          = txprocessor.NewTxProcessor(
			trustedHeaderFetcher, storage, proofSubmitter, logRegistry.Get(TxProcessorContext), cfg.CheckSubmittedTxStatusDelay)
		kvProcessor = kvprocessor.NewKVProcessor(
			trustedHeaderFetcher,
			targetQuerier,
			cfg.MinKvUpdatePeriod,
			logRegistry.Get(KVProcessorContext),
			proofSubmitter,
			storage,
			targetChain,
			neutronChain,
		)
		relayer = relay.NewRelayer(
			cfg,
			txQuerier,
			storage,
			txProcessor,
			kvProcessor,
			targetChain,
			logRegistry.Get(RelayerContext),
		)
	)
	return relayer, nil
}

func NewDefaultStorage(cfg config.NeutronQueryRelayerConfig) (relay.Storage, error) {
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

func loadNeutronChain(cfg config.NeutronQueryRelayerConfig,
	keybase keyring.Keyring, keyName string, logRegistry *nlogger.Registry, connParams *connectionParams,
) (neutronChain *cosmosrelayer.Chain, err error) {
	neutronChain, err = relay.GetNeutronChain(logRegistry.Get(NeutronChainProviderContext), cfg.NeutronChain, connParams.neutronChainID, keyName)

	if err != nil {
		return nil, fmt.Errorf("failed to load neutron chain from env: %w", err)
	}

	if err := neutronChain.AddPath(connParams.neutronClientID, cfg.NeutronChain.ConnectionID); err != nil {
		return nil, fmt.Errorf("failed to AddPath to destination chain: %w", err)
	}

	if err := neutronChain.ChainProvider.Init(); err != nil {
		return nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	// Workaround to have more flexibility in keyring configuration
	// Unfortunately, ChainProvider interface doesn't allow configuring keyring properly or passing existing one
	provConcrete, ok := neutronChain.ChainProvider.(*cosmos.CosmosProvider)
	if !ok {
		return nil, fmt.Errorf("failed to patch CosmosProvider config (type cast failed)")
	}
	provConcrete.Keybase = keybase

	return neutronChain, nil
}

func loadTargetChain(cfg config.NeutronQueryRelayerConfig,
	logRegistry *nlogger.Registry, connParams *connectionParams,
) (targetChain *cosmosrelayer.Chain, err error) {
	targetChain, err = relay.GetTargetChain(logRegistry.Get(TargetChainProviderContext), cfg.TargetChain, connParams.targetChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to load target chain from env: %w", err)
	}

	if err := targetChain.AddPath(connParams.targetClientID, connParams.targetConnectionID); err != nil {
		return nil, fmt.Errorf("failed to AddPath to source chain: %w", err)
	}

	if err := targetChain.ChainProvider.Init(); err != nil {
		return nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	return targetChain, nil
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
