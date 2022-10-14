package app

import (
	"context"
	"fmt"

	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"go.uber.org/zap"

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

func NewDefaultSubscriber(cfg config.NeutronQueryRelayerConfig, logger *zap.Logger) relay.Subscriber {
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
		logger,
	)
	if err != nil {
		logger.Fatal("failed to get a NewSubscriber", zap.Error(err))
	}

	return subscriber
}

func NewDefaultTxSubmitChecker(cfg config.NeutronQueryRelayerConfig, logger *zap.Logger) relay.TxSubmitChecker {
	relayerStorage, err := loadRelayerStorage(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to loadRelayerStorage", zap.Error(err))
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout)
	if err != nil {
		logger.Fatal("cannot create neutron client", zap.Error(err))
	}

	return txsubmitchecker.NewTxSubmitChecker(
		relayerStorage,
		neutronClient,
		logger,
	)
}

// NewDefaultRelayer returns a relayer built with cfg.
func NewDefaultRelayer(ctx context.Context, cfg config.NeutronQueryRelayerConfig, logger *zap.Logger) *relay.Relayer {
	logger.Info("initialized config")
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddr, cfg.TargetChain.Timeout, logger)
	if err != nil {
		logger.Fatal("could not initialize target rpc client", zap.Error(err))
	}

	targetQuerier, err := tmquerier.NewQuerier(targetClient, cfg.TargetChain.ChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		logger.Fatal("cannot connect to target chain", zap.Error(err))
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout, logger)
	if err != nil {
		logger.Fatal("cannot create neutron client", zap.Error(err))
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.NeutronChain.ChainID, cfg.NeutronChain.HomeDir)
	if err != nil {
		logger.Fatal("cannot initialize keybase", zap.Error(err))
	}

	txSender, err := submit.NewTxSender(ctx, neutronClient, codec.Marshaller, keybase, *cfg.NeutronChain, logger)
	if err != nil {
		logger.Fatal("cannot create tx sender", zap.Error(err))
	}

	relayerStorage, err := loadRelayerStorage(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to loadRelayerStorage", zap.Error(err))
	}

	neutronChain, targetChain, err := loadChains(cfg, logger)
	if err != nil {
		logger.Error("failed to loadChains", zap.Error(err))
	}

	var (
		proofSubmitter       = submit.NewSubmitterImpl(txSender, cfg.AllowKVCallbacks, neutronChain.PathEnd.ClientID)
		txQuerier            = txquerier.NewTXQuerySrv(targetQuerier.Client)
		trustedHeaderFetcher = trusted_headers.NewTrustedHeaderFetcher(neutronChain, targetChain, logger)
		txProcessor          = txprocessor.NewTxProcessor(trustedHeaderFetcher, relayerStorage, proofSubmitter, logger, cfg.CheckSubmittedTxStatusDelay)
		kvProcessor          = kvprocessor.NewKVProcessor(
			targetQuerier,
			cfg.MinKvUpdatePeriod,
			logger,
			proofSubmitter,
			relayerStorage,
			targetChain,
			neutronChain,
		)
		relayer = relay.NewRelayer(
			cfg,
			txQuerier,
			relayerStorage,
			txProcessor,
			kvProcessor,
			logger,
		)
	)
	return relayer
}

func loadChains(
	cfg config.NeutronQueryRelayerConfig,
	logger *zap.Logger,
) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
	targetChain, err = relay.GetTargetChain(logger, cfg.TargetChain)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load target chain from env: %w", err)
	}

	if err := targetChain.AddPath(cfg.TargetChain.ClientID, cfg.TargetChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to source chain: %w", err)
	}

	if err := targetChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	neutronChain, err = relay.GetNeutronChain(logger, cfg.NeutronChain)

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

func loadRelayerStorage(cfg config.NeutronQueryRelayerConfig, logger *zap.Logger) (relay.Storage, error) {
	var (
		err            error
		relayerStorage relay.Storage
	)

	if cfg.AllowTxQueries && cfg.StoragePath == "" {
		logger.Fatal("RELAYER_DB_PATH must be set with RELAYER_ALLOW_TX_QUERIES=true")
	}

	if cfg.StoragePath != "" {
		relayerStorage, err = storage.NewLevelDBStorage(cfg.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("couldn't initialize levelDB storage: %w", err)
		}
	} else {
		relayerStorage = storage.NewDummyStorage()
	}

	return relayerStorage, nil
}
