package app

import (
	"context"
	"fmt"

	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	"github.com/neutron-org/neutron-query-relayer/internal/kvprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/storage"
	"github.com/neutron-org/neutron-query-relayer/internal/submit"
	relaysubscriber "github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	"github.com/neutron-org/neutron-query-relayer/internal/trusted_headers"
	"github.com/neutron-org/neutron-query-relayer/internal/txprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/txquerier"
	"github.com/neutron-org/neutron-query-relayer/internal/txsubmitchecker"
	neutronapp "github.com/neutron-org/neutron/app"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

const (
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

// NewDefaultRelayer returns a relayer built with cfg.
func NewDefaultRelayer(
	ctx context.Context,
	cfg config.NeutronQueryRelayerConfig,
	logRegistry *nlogger.Registry,
) (*relay.Relayer, error) {
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddr, cfg.TargetChain.Timeout, logRegistry.Get(TargetChainRPCClientContext))
	if err != nil {
		return nil, fmt.Errorf("could not initialize target rpc client: %w", err)
	}

	targetQuerier, err := tmquerier.NewQuerier(targetClient, cfg.TargetChain.ChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to target chain: %w", err)
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout, logRegistry.Get(NeutronChainRPCClientContext))
	if err != nil {
		return nil, fmt.Errorf("cannot create neutron client: %w", err)
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.NeutronChain.ChainID, cfg.NeutronChain.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize keybase: %w", err)
	}

	txSender, err := submit.NewTxSender(ctx, neutronClient, codec.Marshaller, keybase, *cfg.NeutronChain, logRegistry.Get(TxSenderContext))
	if err != nil {
		return nil, fmt.Errorf("cannot create tx sender: %w", err)
	}

	var st relay.Storage

	if cfg.AllowTxQueries && cfg.StoragePath == "" {
		return nil, fmt.Errorf("RELAYER_DB_PATH must be set with RELAYER_ALLOW_TX_QUERIES=true")
	}

	if cfg.StoragePath != "" {
		st, err = storage.NewLevelDBStorage(cfg.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("couldn't initialize levelDB storage: %w", err)
		}
	} else {
		st = storage.NewDummyStorage()
	}

	neutronChain, targetChain, err := loadChains(cfg, logRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to loadChains: %w", err)
	}

	var (
		proofSubmitter       = submit.NewSubmitterImpl(txSender, cfg.AllowKVCallbacks, neutronChain.PathEnd.ClientID)
		txQuerier            = txquerier.NewTXQuerySrv(targetQuerier.Client)
		trustedHeaderFetcher = trusted_headers.NewTrustedHeaderFetcher(neutronChain, targetChain, logRegistry.Get(TrustedHeadersFetcherContext))
		txProcessor          = txprocessor.NewTxProcessor(trustedHeaderFetcher, st, proofSubmitter, logRegistry.Get(TxProcessorContext))
		kvProcessor          = kvprocessor.NewKVProcessor(
			targetQuerier,
			cfg.MinKvUpdatePeriod,
			logRegistry.Get(KVProcessorContext),
			proofSubmitter,
			st,
			targetChain,
			neutronChain,
		)
		txSubmitChecker = txsubmitchecker.NewTxSubmitChecker(
			txProcessor.GetSubmitNotificationChannel(),
			st,
			neutronClient,
			logRegistry.Get(TxSubmitCheckerContext),
			cfg.CheckSubmittedTxStatusDelay,
		)
		relayer = relay.NewRelayer(
			cfg,
			txQuerier,
			st,
			txProcessor,
			kvProcessor,
			txSubmitChecker,
			targetChain,
			logRegistry.Get(RelayerContext),
		)
	)
	return relayer, nil
}

func loadChains(cfg config.NeutronQueryRelayerConfig, logRegistry *nlogger.Registry) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
	targetChain, err = relay.GetTargetChain(logRegistry.Get(TargetChainProviderContext), cfg.TargetChain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load target chain from env: %w", err)
	}

	if err := targetChain.AddPath(cfg.TargetChain.ClientID, cfg.TargetChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to source chain: %w", err)
	}

	if err := targetChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	neutronChain, err = relay.GetNeutronChain(logRegistry.Get(NeutronChainProviderContext), cfg.NeutronChain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load neutron chain from env: %w", err)
	}

	if err := neutronChain.AddPath(cfg.NeutronChain.ClientID, cfg.NeutronChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to destination chain: %w", err)
	}

	if err := neutronChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	return neutronChain, targetChain, nil
}
