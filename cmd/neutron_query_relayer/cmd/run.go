package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	neutronapp "github.com/neutron-org/neutron/app"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/app"
	"github.com/neutron-org/neutron-query-relayer/internal/config"
)

const (
	mainContext     = "main"
	QueryIdFlagName = "query_id"
)

var queryIds []string

// RunCmd represents the start command
var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the query relayer main app",
	RunE: func(cmd *cobra.Command, args []string) error {
		queryIds, err := cmd.Flags().GetStringSlice(QueryIdFlagName)
		fmt.Println(queryIds)
		if len(queryIds) == 0 {
			return fmt.Errorf("empty list of query ids to relay")
		}
		if err != nil {
			return err
		}

		return startRelayer(queryIds)
	},
}

func init() {
	RunCmd.PersistentFlags().StringSliceVarP(&queryIds, QueryIdFlagName, "q", []string{}, "list of query ids to relay")
	rootCmd.AddCommand(RunCmd)
}

func startRelayer(queryIds []string) error {
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()

	logRegistry, err := nlogger.NewRegistry(
		mainContext,
		app.AppContext,
		app.SubscriberContext,
		app.RelayerContext,
		app.TargetChainRPCClientContext,
		app.NeutronChainRPCClientContext,
		app.TargetChainProviderContext,
		app.NeutronChainProviderContext,
		app.TxSenderContext,
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

	subscriber, err := app.NewDefaultSubscriber(cfg, logRegistry)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultSubscriber", zap.Error(err))
	}

	deps, err := app.NewDefaultDependencyContainer(ctx, cfg, logRegistry, storage)
	if err != nil {
		logger.Fatal("failed to initialize dependency container", zap.Error(err))
	}

	kvprocessor, err := app.NewDefaultKVProcessor(logRegistry, storage, deps)
	if err != nil {
		logger.Fatal("Failed to get NewDefaultKVProcessor", zap.Error(err))
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		// The subscriber writes to the tasks queue.
		for _, queryId := range queryIds {
			query, err := subscriber.GetNeutronRegisteredQuery(ctx, queryId)
			if err != nil {
				logger.Error("could not getNeutronRegisteredQueries: %w", zap.Error(err))
			}

			fmt.Println(query)

			msg := &relay.MessageKV{QueryId: query.Id, KVKeys: query.Keys}
			kvprocessor.ProcessAndSubmit(ctx, msg)
		}

		fmt.Println("end of submission")
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

	return nil
}
