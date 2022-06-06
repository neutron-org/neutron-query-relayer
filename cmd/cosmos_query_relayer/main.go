package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"

	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/lidofinance/cosmos-query-relayer/internal/relay"
	"github.com/lidofinance/cosmos-query-relayer/internal/submit"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"

	"github.com/lidofinance/cosmos-query-relayer/internal/config"
	"github.com/lidofinance/cosmos-query-relayer/internal/raw"
)

const configPathEnv = "CONFIG_PATH"

func main() {
	fmt.Println("cosmos-query-relayer starts...")

	ctx := context.Background()
	cfgPath := os.Getenv(configPathEnv)
	cfg, err := config.NewCosmosQueryRelayerConfig(cfgPath)
	if err != nil {
		log.Fatalf("cannot initialize relayer config: %s", err)
	}
	fmt.Println("initialized config")

	raw.SetSDKConfig(cfg.LidoChain.ChainPrefix)

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddress, cfg.TargetChain.Timeout)
	if err != nil {
		log.Fatalf("could not initialize target rpc client: %s", err)
	}

	targetQuerier, err := proof.NewQuerier(targetClient, cfg.TargetChain.ChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %s", err)
	}

	lidoClient, err := raw.NewRPCClient(cfg.LidoChain.RPCAddress, cfg.LidoChain.Timeout)
	if err != nil {
		log.Fatalf("cannot create lido client: %s", err)
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.LidoChain.ChainID, cfg.LidoChain.HomeDir)
	if err != nil {
		log.Fatalf("cannot initialize keybase: %s", err)
	}

	txSender, err := submit.NewTxSender(lidoClient, codec.Marshaller, keybase, cfg.LidoChain)
	if err != nil {
		log.Fatalf("cannot create tx sender: %s", err)
	}

	proofSubmitter := submit.NewSubmitterImpl(txSender)
	proofFetcher := proof_impl.NewProofer(targetQuerier)

	logger := zap.NewExample() // TODO: add proper logging.

	targetChain, err := relay.GetChainFromFile(logger, cfg.TargetChain.HomeDir,
		cfg.TargetChain.ChainProviderConfigPath, cfg.TargetChain.Debug)
	if err != nil {
		log.Fatalf("failed to GetChainFromFile %s: %s", cfg.TargetChain.ChainProviderConfigPath, err)
	}

	if err := targetChain.AddPath(cfg.TargetChain.ClientID, cfg.TargetChain.ConnectionID); err != nil {
		log.Fatalf("failed to AddPath to source chain: %s", err)
	}

	if err := targetChain.ChainProvider.Init(); err != nil {
		log.Fatalf("failed to Init source chain provider: %s", err)
	}

	lidoChain, err := relay.GetChainFromFile(logger, cfg.LidoChain.HomeDir,
		cfg.LidoChain.ChainProviderConfigPath, cfg.LidoChain.Debug)
	if err != nil {
		log.Fatalf("failed to GetChainFromFile %s: %s", cfg.LidoChain.ChainProviderConfigPath, err)
	}

	if err := lidoChain.AddPath(cfg.LidoChain.ClientID, cfg.LidoChain.ConnectionID); err != nil {
		log.Fatalf("failed to AddPath to destination chain: %s", err)
	}

	if err := lidoChain.ChainProvider.Init(); err != nil {
		log.Fatalf("failed to Init source chain provider: %s", err)
	}

	relayer := relay.NewRelayer(
		proofFetcher,
		proofSubmitter,
		cfg.TargetChain.ChainID,
		cfg.TargetChain.AccountPrefix,
		targetChain,
		lidoChain,
		)

	fmt.Println("subscribing to lido chain events")
	// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
	// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
	err = raw.Subscribe(ctx, cfg.TargetChain.ChainID+"-client", cfg.LidoChain.RPCAddress, raw.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		err = relayer.Proof(ctx, event)
		if err != nil {
			fmt.Printf("error proofing event: %s\n", err)
		}
	})
	if err != nil {
		log.Fatalf("error subscribing to lido chain events: %s", err)
	}
}
