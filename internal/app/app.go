package app

import (
	"fmt"
	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	"github.com/neutron-org/neutron-query-relayer/internal/proof"
	"github.com/neutron-org/neutron-query-relayer/internal/proof/proof_impl"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/storage"
	"github.com/neutron-org/neutron-query-relayer/internal/submit"
	"github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/txprocessor"
	neutronapp "github.com/neutron-org/neutron/app"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"go.uber.org/zap"
)

func loadChains(cfg config.NeutronQueryRelayerConfig, logger *zap.Logger) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
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

func NewDefaultRelayer(logger *zap.Logger, cfg config.NeutronQueryRelayerConfig) *relay.Relayer {

	logger.Info("initialized config")
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddr, cfg.TargetChain.Timeout)
	if err != nil {
		logger.Fatal("could not initialize target rpc client", zap.Error(err))
	}

	targetQuerier, err := proof.NewQuerier(targetClient, cfg.TargetChain.ChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		logger.Fatal("cannot connect to target chain", zap.Error(err))
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout)
	if err != nil {
		logger.Fatal("cannot create neutron client", zap.Error(err))
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.NeutronChain.ChainID, cfg.NeutronChain.HomeDir)
	if err != nil {
		logger.Fatal("cannot initialize keybase", zap.Error(err))
	}

	txSender, err := submit.NewTxSender(neutronClient, codec.Marshaller, keybase, *cfg.NeutronChain, logger)
	if err != nil {
		logger.Fatal("cannot create tx sender", zap.Error(err))
	}

	proofSubmitter := submit.NewSubmitterImpl(txSender)
	proofFetcher := proof_impl.NewProofer(targetQuerier)
	neutronChain, targetChain, err := loadChains(cfg, logger)
	if err != nil {
		logger.Error("failed to loadChains", zap.Error(err))
	}

	var st relay.Storage

	if cfg.AllowTxQueries && cfg.StoragePath == "" {
		logger.Fatal("RELAYER_DB_PATH must be set with RELAYER_ALLOW_TX_QUERIES=true")
	}

	if cfg.StoragePath != "" {
		st, err = storage.NewLevelDBStorage(cfg.StoragePath)
		if err != nil {
			logger.Fatal("couldn't initialize levelDB storage", zap.Error(err))
		}
	} else {
		st = storage.NewDummyStorage()
	}

	txQuerier := proof_impl.NewTXQuerySrv(targetQuerier.Client)
	watchedMsgTypes := []neutrontypes.InterchainQueryType{neutrontypes.InterchainQueryTypeKV}
	if cfg.AllowTxQueries {
		watchedMsgTypes = append(watchedMsgTypes, neutrontypes.InterchainQueryTypeTX)
	}
	subscriber, err := subscriber.NewSubscriber(
		cfg.NeutronChain.RPCAddr,
		cfg.TargetChain.ChainID,
		registry.New(cfg.Registry),
		watchedMsgTypes,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to init subscriber", zap.Error(err))
	}

	csManager := relay.NewConsensusStatesManager(targetChain, neutronChain)

	txProcessor := txprocessor.NewTxProcessor(csManager, st, proofSubmitter, neutronChain.PathEnd.ClientID, logger)

	relayer := relay.NewRelayer(
		cfg,
		proofFetcher,
		txQuerier,
		proofSubmitter,
		targetChain,
		neutronChain,
		subscriber,
		logger,
		st,
		txProcessor,
	)
	return relayer
}
