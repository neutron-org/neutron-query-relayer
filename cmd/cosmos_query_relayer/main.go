package main

import (
	"context"
	"fmt"
	"log"
	"os"

	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/neutron-org/cosmos-query-relayer/internal/raw"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
	"github.com/neutron-org/cosmos-query-relayer/internal/submit"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
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

	raw.SetSDKConfig(cfg.NeutronChain.ChainPrefix)

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddress, cfg.TargetChain.Timeout)
	if err != nil {
		log.Fatalf("could not initialize target rpc client: %s", err)
	}

	targetQuerier, err := proof.NewQuerier(targetClient, cfg.TargetChain.ChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		log.Fatalf("cannot connect to target chain: %s", err)
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddress, cfg.NeutronChain.Timeout)
	if err != nil {
		log.Fatalf("cannot create neutron client: %s", err)
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(cfg.NeutronChain.ChainID, cfg.NeutronChain.HomeDir)
	if err != nil {
		log.Fatalf("cannot initialize keybase: %s", err)
	}

	txSender, err := submit.NewTxSender(neutronClient, codec.Marshaller, keybase, cfg.NeutronChain)
	if err != nil {
		log.Fatalf("cannot create tx sender: %s", err)
	}

	proofSubmitter := submit.NewSubmitterImpl(txSender)
	proofFetcher := proof_impl.NewProofer(targetQuerier)

	logger := zap.NewExample() // TODO: add proper logging.

	neutronChain, targetChain, err := loadChains(cfg, logger)
	if err != nil {
		log.Fatalf("failed to loadChains: %s", err)
	}

	relayer := relay.NewRelayer(
		proofFetcher,
		proofSubmitter,
		cfg.TargetChain.ChainID,
		cfg.TargetChain.AccountPrefix,
		targetChain,
		neutronChain,
	)

	fmt.Println("subscribing to neutron chain events")
	// NOTE: no parallel processing here. What if proofs or transaction submissions for each event will take too long?
	// Then the proofs will be for past events, but still for last target blockchain state, and that is kinda okay for now
	err = raw.Subscribe(ctx, cfg.TargetChain.ChainID+"-client", cfg.NeutronChain.RPCAddress, raw.SubscribeQuery(cfg.TargetChain.ChainID), func(event coretypes.ResultEvent) {
		err = relayer.Proof(ctx, event)
		if err != nil {
			fmt.Printf("error proofing event: %s\n", err)
		}
	})
	if err != nil {
		log.Fatalf("error subscribing to neutron chain events: %s", err)
	}
}

func loadChains(cfg config.CosmosQueryRelayerConfig, logger *zap.Logger) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
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

	neutronChain, err = relay.GetChainFromFile(logger, cfg.NeutronChain.HomeDir,
		cfg.NeutronChain.ChainProviderConfigPath, cfg.NeutronChain.Debug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to GetChainFromFile %s: %w", cfg.NeutronChain.ChainProviderConfigPath, err)
	}

	if err := neutronChain.AddPath(cfg.NeutronChain.ClientID, cfg.NeutronChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to destination chain: %w", err)
	}

	if err := neutronChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	return neutronChain, targetChain, nil
}
