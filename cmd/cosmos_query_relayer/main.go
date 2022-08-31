package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/neutron-org/cosmos-query-relayer/internal/config"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof/proof_impl"
	"github.com/neutron-org/cosmos-query-relayer/internal/raw"
	"github.com/neutron-org/cosmos-query-relayer/internal/registry"
	"github.com/neutron-org/cosmos-query-relayer/internal/relay"
	"github.com/neutron-org/cosmos-query-relayer/internal/storage"
	"github.com/neutron-org/cosmos-query-relayer/internal/submit"
	"github.com/neutron-org/cosmos-query-relayer/internal/subscriber"
	neutronapp "github.com/neutron-org/neutron/app"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
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

	var store relay.Storage

	if cfg.AllowTxQueries && cfg.StoragePath == "" {
		logger.Fatal("path to relayer's storage must be set, please refer to the README.md for more information about env variables")
	}

	if cfg.StoragePath != "" {
		store, err = storage.NewLevelDBStorage(cfg.StoragePath)
		if err != nil {
			logger.Fatal("couldn't initialize levelDB storage", zap.Error(err))
		}
	} else {
		store = storage.NewDummyStorage()
	}

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

	relayer := relay.NewRelayer(
		cfg,
		proofFetcher,
		proofSubmitter,
		targetChain,
		neutronChain,
		subscriber,
		logger,
		store,
	)

	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go runRelayer(relayer, logger, wg, ctx, errChan)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-sigs:
		logger.Info("received an OS signal, gracefully shutting down...", zap.String("signal", s.String()))
		cancel()
		wg.Wait()
	case err := <-errChan:
		logger.Error(err.Error())
		os.Exit(1)
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

// runRelayer starts a background relaying process and manages its execution and a possible error
// result. If the ctx is closed, a possible Run error is sent to the error channel. Also, if an
// error occurs before the context is closed, it's sent to the error channel as well.
func runRelayer(
	relayer *relay.Relayer,
	logger *zap.Logger,
	wg *sync.WaitGroup,
	ctx context.Context,
	errChan chan<- error,
) {
	defer wg.Done()
	if err := relayer.Run(ctx); err != nil {
		select {
		case <-ctx.Done():
			logger.Error("relayer process finished with an error", zap.Error(err))
		case errChan <- fmt.Errorf("relayer process finished with an error: %w", err):
		}
	}
}
