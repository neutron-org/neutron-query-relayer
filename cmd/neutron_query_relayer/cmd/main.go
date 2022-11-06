package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/neutron-org/neutron-query-relayer/internal/storage"
	"github.com/neutron-org/neutron-query-relayer/internal/webserver"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

const (
	mainContext = "main"
)

func startRelayer() {
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
		webserver.ServerContext,
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
		logger.Info("metrics handler set up")
	}()

	// === TODO: wait until storage is created in main (See oopcode's PR) and remove this!
	var st relay.Storage

	if cfg.AllowTxQueries && cfg.StoragePath == "" {
		//return nil, fmt.Errorf("RELAYER_DB_PATH must be set with RELAYER_ALLOW_TX_QUERIES=true")
	}

	if cfg.StoragePath != "" {
		st, err = storage.NewLevelDBStorage(cfg.StoragePath)
		//if err != nil {
		//	return nil, fmt.Errorf("couldn't initialize levelDB storage: %w", err)
		//}
	} else {
		st = storage.NewDummyStorage()
	}
	// === REMOVE UNTIL HERE

	// TODO: move to separate server (and port)
	go func() {
		router := webserver.Router(logRegistry, st)
		err = http.ListenAndServe(fmt.Sprintf(":%d", cfg.WebserverPort), router)
		if err != nil {
			logger.Fatal("failed to serve webserver", zap.Error(err))
		}
		logger.Info("rest webserver set up")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	subscriber, err := app.NewDefaultSubscriber(cfg, logRegistry)
	if err != nil {
		logger.Fatal("failed to create subscriber", zap.Error(err))
	}
	relayer, err := app.NewDefaultRelayer(st, ctx, cfg, logRegistry)
	if err != nil {
		logger.Fatal("failed to create relayer", zap.Error(err))
	}
	queriesTasksQueue := make(chan neutrontypes.RegisteredQuery, cfg.QueriesTaskQueueCapacity)

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
		if err := relayer.Run(ctx, queriesTasksQueue); err != nil {
			logger.Error("Relayer exited with an error", zap.Error(err))
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
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
