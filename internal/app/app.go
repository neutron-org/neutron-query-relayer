package app

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"strings"

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

func NewDefaultSubscriber(logger *zap.Logger, cfg config.NeutronQueryRelayerConfig) relay.Subscriber {
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
	logger.Debug("Subscribed successfully",
		zap.String("rpc", cfg.NeutronChain.RPCAddr),
		zap.String("rest", cfg.NeutronChain.RESTAddr),
		zap.Strings("registries", cfg.Registry.Addresses),
	)

	return subscriber
}

// NewDefaultRelayer returns a relayer built with cfg.
func NewDefaultRelayer(
	ctx context.Context,
	logger *zap.Logger,
	cfg config.NeutronQueryRelayerConfig,
) *relay.Relayer {
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

	keybase, keyName, err := initializeKeychain(*cfg.NeutronChain)
	if err != nil {
		logger.Fatal("cannot initialize keyring", zap.Error(err))
	}

	txSender, err := submit.NewTxSender(ctx, neutronClient, codec.Marshaller, *cfg.NeutronChain, keybase, keyName, logger)
	if err != nil {
		logger.Fatal("cannot create tx sender", zap.Error(err))
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

	neutronChain, targetChain, err := loadChains(cfg, keyName, logger)
	if err != nil {
		logger.Error("failed to loadChains", zap.Error(err))
	}

	if cfg.NeutronChain.KeyringBackend == keyring.BackendMemory {
		_, err := neutronChain.ChainProvider.RestoreKey(keyName, cfg.NeutronChain.SignKeySeed, sdk.CoinType)
		if err != nil {
			logger.Fatal("cannot add key to the memory keyring", zap.Error(err))
		}
	}

	var (
		proofSubmitter       = submit.NewSubmitterImpl(txSender, cfg.AllowKVCallbacks, neutronChain.PathEnd.ClientID)
		txQuerier            = txquerier.NewTXQuerySrv(targetQuerier.Client)
		trustedHeaderFetcher = trusted_headers.NewTrustedHeaderFetcher(neutronChain, targetChain, logger)
		txProcessor          = txprocessor.NewTxProcessor(trustedHeaderFetcher, st, proofSubmitter, logger)
		kvProcessor          = kvprocessor.NewKVProcessor(
			targetQuerier,
			cfg.MinKvUpdatePeriod,
			logger,
			proofSubmitter,
			st,
			targetChain,
			neutronChain,
		)
		txSubmitChecker = txsubmitchecker.NewTxSubmitChecker(
			txProcessor.GetSubmitNotificationChannel(),
			st,
			neutronClient,
			logger,
			cfg.CheckSubmittedTxStatusDelay,
		)
		relayer = relay.NewRelayer(
			cfg,
			txQuerier,
			st,
			txProcessor,
			kvProcessor,
			txSubmitChecker,
			logger,
		)
	)
	return relayer
}

func loadChains(cfg config.NeutronQueryRelayerConfig, keyName string, logger *zap.Logger) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
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

	neutronChain, err = relay.GetNeutronChain(logger, cfg.NeutronChain, keyName)

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

func initializeKeychain(cfg config.NeutronChainConfig) (keyring.Keyring, string, error) {
	passReader := strings.NewReader(cfg.KeyringPassword)
	keybase, err := keyring.New(cfg.ChainID, cfg.KeyringBackend, cfg.HomeDir, passReader)
	if err != nil {
		return nil, "", fmt.Errorf("error creating keybase for chainId=%s and keyringRootDir=%s: %w", cfg.ChainID, cfg.HomeDir, err)
	}

	keyName := cfg.SignKeyName

	// If the keybase is set to "memory" then we expect the seed to be passed via environment variable and we need to
	// add the key to the in-memory keybase
	if cfg.KeyringBackend == keyring.BackendMemory {
		// For in-memory key we ignore the name provided by user (so it might be (and actually should be) left empty)
		keyName = "sign_key"
		signKeyHdPath := hd.CreateHDPath(sdk.CoinType, 0, 0).String()
		_, err := keybase.NewAccount(keyName, cfg.SignKeySeed, "", signKeyHdPath, hd.Secp256k1)
		if err != nil {
			return nil, "", err
		}
	}
	return keybase, keyName, nil
}
