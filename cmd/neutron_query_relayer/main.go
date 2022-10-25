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

	var (
		queriesTasksQueue = make(chan neutrontypes.RegisteredQuery, cfg.QueriesTaskQueueCapacity)
		subscriber        = app.NewDefaultSubscriber(logger, cfg)
		relayer           = app.NewDefaultRelayer(logger, cfg)
	)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

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
