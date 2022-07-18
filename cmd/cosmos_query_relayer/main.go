package main

import (
	"context"
	"fmt"
	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/lidofinance/cosmos-query-relayer/internal/raw"
	"github.com/lidofinance/cosmos-query-relayer/internal/relay"
	"github.com/lidofinance/cosmos-query-relayer/internal/submit"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
	"net/http"
	"os"
)

const configPathEnv = "CONFIG_PATH"

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("coulnd initialize loggerr: %s", err)
	}
	logger.Info("cosmos-query-relayer starts...")

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil)
	logger.Info("metrics handler set up")

	ctx := context.Background()
	cfgPath := os.Getenv(configPathEnv)
	cfg, err := config.NewCosmosQueryRelayerConfig(cfgPath)
	if err != nil {
		logger.Error(fmt.Sprintf("cannot initialize relayer config: %s", err))
	}
	logger.Info("initialized config")
	raw.SetSDKConfig(cfg.LidoChain.ChainPrefix)

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddress, cfg.TargetChain.Timeout)
	if err != nil {
		logger.Fatal(fmt.Sprintf("could not initialize target rpc client: %s", err))
	}

	targetQuerier, err := proof.NewQuerier(targetClient, cfg.TargetChain.ChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		logger.Fatal(fmt.Sprintf("cannot connect to target chain: %s", err))
	}

	lidoClient, err := raw.NewRPCClient(cfg.LidoChain.RPCAddress, cfg.LidoChain.Timeout)
	if err != nil {
		logger.Fatal(fmt.Sprintf("cannot create lido client: %s", err))
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.LidoChain.ChainID, cfg.LidoChain.HomeDir)
	if err != nil {
		logger.Fatal(fmt.Sprintf("cannot initialize keybase: %s", err))
	}

	txSender, err := submit.NewTxSender(lidoClient, codec.Marshaller, keybase, cfg.LidoChain)
	if err != nil {
		logger.Error(fmt.Sprintf("cannot create tx sender: %s", err))
	}

	proofSubmitter := submit.NewSubmitterImpl(txSender)
	proofFetcher := proof_impl.NewProofer(targetQuerier)

	lidoChain, targetChain, err := loadChains(cfg, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to loadChains: %s", err))
	}

	relayer := relay.NewRelayer(
		proofFetcher,
		proofSubmitter,
		cfg.TargetChain.ChainID,
		cfg.TargetChain.AccountPrefix,
		targetChain,
		lidoChain,
		logger,
	)

	logger.Info("subscribing to lido chain events")
	// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
	// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
	err = raw.Subscribe(ctx, cfg.TargetChain.ChainID+"-client", cfg.LidoChain.RPCAddress, raw.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		err = relayer.Proof(ctx, event)
		if err != nil {
			logger.Info(fmt.Sprintf("error proofing event: %s\n", err))
		}
	})
	if err != nil {
		logger.Fatal(fmt.Sprintf("error subscribing to lido chain events: %s", err))
	}
}

func loadChains(cfg config.CosmosQueryRelayerConfig, logger *zap.Logger) (lidoChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
	targetChain, err = relay.GetChainFromFile(logger, cfg.TargetChain.HomeDir,
		cfg.TargetChain.ChainProviderConfigPath, cfg.TargetChain.Debug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to GetChainFromFile %s: %s", cfg.TargetChain.ChainProviderConfigPath, err)
	}

	if err := targetChain.AddPath(cfg.TargetChain.ClientID, cfg.TargetChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to source chain: %w", err)
	}

	if err := targetChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	lidoChain, err = relay.GetChainFromFile(logger, cfg.LidoChain.HomeDir,
		cfg.LidoChain.ChainProviderConfigPath, cfg.LidoChain.Debug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to GetChainFromFile %s: %w", cfg.LidoChain.ChainProviderConfigPath, err)
	}

	if err := lidoChain.AddPath(cfg.LidoChain.ClientID, cfg.LidoChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to destination chain: %w", err)
	}

	if err := lidoChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	return lidoChain, targetChain, nil
}
