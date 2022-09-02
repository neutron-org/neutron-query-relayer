package main

import (
	"context"
	"fmt"
	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/config"
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

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":9999", nil)
		if err != nil {
			logger.Fatal("failed to serve metrics", zap.Error(err))
		}
	}()
	logger.Info("metrics handler set up")
	cfg, err := config.NewNeutronQueryRelayerConfig()
	if err != nil {
		logger.Fatal("cannot initialize relayer config", zap.Error(err))
	}

	relayer, notifChannel := app.NewDefaultRelayer(logger, cfg)

	// DEMO PURPOSE ONLY
	go func() {
		for n := range notifChannel {
			fmt.Println(n)
		}
	}()
	// DEMO PURPOSE ONLY

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := relayer.Run(ctx); err != nil {
			logger.Error("Relayer exited with an error", zap.Error(err))
		}
	}()

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		s := <-sigs
		logger.Info("Received termination signal, gracefully shutting down...", zap.String("signal", s.String()))
		cancel()
	}()

	wg.Wait()
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
