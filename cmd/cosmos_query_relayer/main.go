package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/neutron-org/cosmos-query-relayer/internal/raw"
	"github.com/neutron-org/cosmos-query-relayer/internal/registry"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
	"github.com/neutron-org/cosmos-query-relayer/internal/submit"
	neutronapp "github.com/neutron-org/neutron/app"
)

func main() {
	loggerConfig, err := config.NewLoggerConfig()
	if err != nil {
		log.Fatalf("couldn't initialize logging config: %s", err)
	}
	logger, err := loggerConfig.Build()
	if err != nil {
		log.Fatalf("couldn't initialize logger: %s", err)
	}
	logger.Info("cosmos-query-relayer starts...")

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":9999", nil)
		if err != nil {
			logger.Fatal("failed to serve metrics", zap.Error(err))
		}
	}()
	logger.Info("metrics handler set up")

	cfg, err := config.NewCosmosQueryRelayerConfig()
	if err != nil {
		logger.Fatal("cannot initialize relayer config", zap.Error(err))
	}
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
		cfg,
		proofFetcher,
		proofSubmitter,
		registry.New(cfg.Registry),
		targetChain,
		neutronChain,
		logger,
	)

	ctx := context.Background()
	logger.Info("subscribing to neutron chain events")
	events, err := raw.Subscribe(ctx, cfg.TargetChain.ChainID+"-client", cfg.NeutronChain.RPCAddr, raw.SubscribeQuery(cfg.TargetChain.ChainID))
	if err != nil {
		logger.Error("failed to subscribe on events", zap.Error(err))
	}
	logger.Info("successfully subscribed to neutron chain events\n")

	for event := range events {
		// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
		// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
		if err = relayer.Proof(ctx, event); err != nil {
			logger.Error("failed to prove event on query", zap.String("query", event.Query), zap.Error(err))
		}
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
