package main

import (
	"context"
	"fmt"
	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/neutron-org/cosmos-query-relayer/internal/raw"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
	"github.com/neutron-org/cosmos-query-relayer/internal/submit"
	neutronapp "github.com/neutron-org/neutron/app"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
	"net/http"
	"os"
)

func main() {
	loggerConfig, err := config.NewLoggerConfig()
	if err != nil {
		fmt.Printf("couldn't initialize logging config: %s", err)
		os.Exit(1)
	}
	logger, err := loggerConfig.Build()
	if err != nil {
		fmt.Printf("couldn't initialize logger: %s", err)
		os.Exit(1)
	}
	logger.Info("cosmos-query-relayer starts...")

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":8088", nil)
		if err != nil {
			logger.Error("failed to serve metrics", zap.Error(err))
			os.Exit(1)
		}
	}()
	logger.Info("metrics handler set up")

	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		logger.Error("cannot initialize relayer config", zap.Error(err))
	}
	logger.Info("initialized config", zap.Any("config", cfg))
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()
	logger.Info("config: ", zap.Any("config exported", cfg))

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

	txSender, err := submit.NewTxSender(neutronClient, codec.Marshaller, keybase, *cfg.NeutronChain)
	if err != nil {
		logger.Fatal("cannot create tx sender", zap.Error(err))
	}

	proofSubmitter := submit.NewSubmitterImpl(txSender)
	proofFetcher := proof_impl.NewProofer(targetQuerier)
	neutronChain, targetChain, err := loadChains(cfg, logger)
	if err != nil {
		logger.Error("failed to loadChains", zap.Error(err))
	}

	relayer := relay.NewRelayer(
		proofFetcher,
		proofSubmitter,
		cfg.TargetChain.ChainID,
		cfg.TargetChain.AccountPrefix,
		targetChain,
		neutronChain,
		logger,
	)
	ctx := context.Background()
	logger.Info("subscribing to neutron chain events")
	// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
	// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
	err = raw.Subscribe(ctx, cfg.TargetChain.ChainID+"-client", cfg.NeutronChain.RPCAddr, raw.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		err = relayer.Proof(ctx, event)
		if err != nil {
			logger.Info("error proofing event", zap.Error(err))
		}
	})
	if err != nil {
		logger.Fatal("error subscribing to neutron chain events", zap.Error(err))
	}
}

func loadChains(cfg config.CosmosQueryRelayerConfig, logger *zap.Logger) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
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
