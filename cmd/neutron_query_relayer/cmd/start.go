package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	relaysubscriber "github.com/neutron-org/neutron-query-relayer/internal/subscriber"

	neutronappconfig "github.com/neutron-org/neutron/v4/app/config"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	icqhttp "github.com/neutron-org/neutron-query-relayer/internal/http"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/v4/x/interchainqueries/types"
)

const (
	mainContext = "main"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the query relayer main app",
	Run: func(cmd *cobra.Command, args []string) {
		startRelayer()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func startRelayer() {
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronappconfig.GetDefaultConfig()

	logRegistry, err := nlogger.NewRegistry(
		mainContext,
		app.AppContext,
		app.SubscriberContext,
		app.RelayerContext,
		icqhttp.ServerContext,
		app.TargetChainRPCClientContext,
		app.NeutronChainRPCClientContext,
		app.TargetChainProviderContext,
		app.NeutronChainProviderContext,
		app.TxSenderContext,
		app.TxProcessorContext,
		app.TxSubmitCheckerContext,
		app.TrustedHeadersFetcherContext,
		app.KVProcessorContext,
		icqhttp.MonitoringLoggerContext,
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

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	// The storage has to be shared because of the LevelDB single process restriction.
	storage, err := app.NewDefaultStorage(cfg, logger)
	if err != nil {
		logger.Fatal("failed to create NewDefaultStorage", zap.Error(err))
	}
	defer func(storage relay.Storage) {
		if err := storage.Close(); err != nil {
			logger.Error("failed to close storage", zap.Error(err))
		}
	}(storage)

	var (
		queriesTasksQueue      = make(chan neutrontypes.RegisteredQuery, cfg.QueriesTaskQueueCapacity)
		submittedTxsTasksQueue = make(chan relay.PendingSubmittedTxInfo)
	)

	subscriber, err := relaysubscriber.NewDefaultSubscriber(cfg, logRegistry)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultSubscriber", zap.Error(err))
	}

	deps, err := app.NewDefaultDependencyContainer(ctx, cfg, logRegistry, storage)
	if err != nil {
		logger.Fatal("failed to initialize dependency container", zap.Error(err))
	}

	relayer, err := app.NewDefaultRelayer(cfg, logRegistry, storage, deps)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultRelayer", zap.Error(err))
	}

	txSubmitChecker, err := app.NewDefaultTxSubmitChecker(cfg, logRegistry, storage)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultTxSubmitChecker", zap.Error(err))
	}

	globalCfg.Seal()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := icqhttp.Run(ctx, logRegistry, storage, deps.GetTxProcessor(), submittedTxsTasksQueue, cfg.ListenAddr)
		if err != nil {
			logger.Error("WebServer exited with an error", zap.Error(err))
			cancel()
		}
	}()

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
