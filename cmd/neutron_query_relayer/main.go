package main

import (
	"context"
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
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
	storage, err := app.NewRelayerStorage(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to loadRelayerStorage", zap.Error(err))
	}
	defer func(storage relay.Storage) {
		err := storage.Close()
		if err != nil {
			logger.Fatal("Failed to close storage", zap.Error(err))
		}
	}(storage)

	var (
		queriesTasksQueue      = make(chan neutrontypes.RegisteredQuery, cfg.QueriesTaskQueueCapacity)
		submittedTxsTasksQueue = make(chan relay.PendingSubmittedTxInfo)
		subscriber             = app.NewDefaultSubscriber(cfg, logger)
		txSubmitChecker        = app.NewDefaultTxSubmitChecker(cfg, logger, storage)
		relayer                = app.NewDefaultRelayer(ctx, cfg, logger, storage)
	)

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
