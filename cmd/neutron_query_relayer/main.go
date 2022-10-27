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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

const (
	mainContext = "main"
)

func main() {
	logRegistry, err := nlogger.NewRegistry(
		mainContext,
		app.SubscriberContext,
		app.RelayerContext,
		app.TargetChainRPCClientContext,
		app.NeutronChainRPCClientContext,
		app.TargetChainProviderContext,
		app.NeutronChainProviderContext,
		app.TxSenderContext,
		app.TxProcessorContext,
		app.TxSubmitCheckerContext,
		app.TrustedHeadersFetcherContext,
		app.KVProcessorContext,
	)
	if err != nil {
		log.Fatalf("couldn't initialize loggers registry: %s", err)
	}
	logger := logRegistry.Get(mainContext)
	logger.Info("neutron-query-relayer starts...")

	cfg, err := config.NewNeutronQueryRelayerConfig()
	if err != nil {
		logger.Fatal("cannot initialize relayer config", zap.Error(err))
	}

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.PrometheusPort), nil)
		if err != nil {
			logger.Fatal("failed to serve metrics", zap.Error(err))
		}
	}()
	logger.Info("metrics handler set up")

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	// The storage has to be shared because of the LevelDB single process restriction.
	storage, err := app.NewDefaultStorage(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create NewDefaultStorage", zap.Error(err))
	}
	defer func(storage relay.Storage) {
		if cfg.AllowTxQueries {
			if err := storage.Close(); err != nil {
				logger.Error("Failed to close storage", zap.Error(err))
			}
		}
	}(storage)

	var (
		queriesTasksQueue      = make(chan neutrontypes.RegisteredQuery, cfg.QueriesTaskQueueCapacity)
		submittedTxsTasksQueue = make(chan relay.PendingSubmittedTxInfo)
	)

	subscriber, err := app.NewDefaultSubscriber(cfg, logRegistry)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultSubscriber", zap.Error(err))
	}

	relayer, err := app.NewDefaultRelayer(ctx, cfg, logRegistry, storage)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultRelayer", zap.Error(err))
	}

	txSubmitChecker, err := app.NewDefaultTxSubmitChecker(cfg, logRegistry, storage)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultTxSubmitChecker", zap.Error(err))
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := txSubmitChecker.Run(ctx, submittedTxsTasksQueue)
		if err != nil {
			logger.Error("TxSubmitChecker exited with an error", zap.Error(err))
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		// The subscriber writes to the tasks queue.
		if err := subscriber.Subscribe(ctx, queriesTasksQueue); err != nil {
			logger.Error("Subscriber exited with an error", zap.Error(err))
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		// The relayer reads from the tasks queue.
		if err := relayer.Run(ctx, queriesTasksQueue, submittedTxsTasksQueue); err != nil {
			logger.Error("Relayer exited with an error", zap.Error(err))
			cancel()
		}
	}()

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		s := <-sigs
		logger.Info("Received termination signal, gracefully shutting down...",
			zap.String("signal", s.String()))
		cancel()
	}()

	wg.Wait()
}
