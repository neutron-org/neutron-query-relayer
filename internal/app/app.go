package app

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/neutron-org/neutron-query-relayer/internal/subscriber/querier/client/query"

	rpcclienthttp "github.com/tendermint/tendermint/rpc/client/http"

	cosmosrelayer "github.com/cosmos/relayer/v2/relayer"
	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/config"
	"github.com/neutron-org/neutron-query-relayer/internal/kvprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/raw"
	"github.com/neutron-org/neutron-query-relayer/internal/registry"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/neutron-org/neutron-query-relayer/internal/storage"
	"github.com/neutron-org/neutron-query-relayer/internal/submit"
	relaysubscriber "github.com/neutron-org/neutron-query-relayer/internal/subscriber"
	"github.com/neutron-org/neutron-query-relayer/internal/tmquerier"
	"github.com/neutron-org/neutron-query-relayer/internal/trusted_headers"
	"github.com/neutron-org/neutron-query-relayer/internal/txprocessor"
	"github.com/neutron-org/neutron-query-relayer/internal/txquerier"
	"github.com/neutron-org/neutron-query-relayer/internal/txsubmitchecker"
	neutronapp "github.com/neutron-org/neutron/app"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// retries configuration for fetching connection info
var (
	RtyAtt = retry.Attempts(uint(5))
	RtyDel = retry.Delay(time.Millisecond * 10000)
	RtyErr = retry.LastErrorOnly(true)
)

func NewDefaultSubscriber(logger *zap.Logger, cfg config.NeutronQueryRelayerConfig) relay.Subscriber {
	watchedMsgTypes := []neutrontypes.InterchainQueryType{neutrontypes.InterchainQueryTypeKV}
	if cfg.AllowTxQueries {
		watchedMsgTypes = append(watchedMsgTypes, neutrontypes.InterchainQueryTypeTX)
	}

	subscriber, err := relaysubscriber.NewSubscriber(
		cfg.NeutronChain.RPCAddr,
		cfg.NeutronChain.RESTAddr,
		cfg.NeutronChain.ConnectionID,
		registry.New(cfg.Registry),
		watchedMsgTypes,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to get a NewSubscriber", zap.Error(err))
	}

	return subscriber
}

// NewDefaultRelayer returns a relayer built with cfg.
func NewDefaultRelayer(
	ctx context.Context,
	logger *zap.Logger,
	cfg config.NeutronQueryRelayerConfig,
) *relay.Relayer {
	logger.Info("initialized config")
	// set global values for prefixes for cosmos-sdk when parsing addresses and so on
	globalCfg := neutronapp.GetDefaultConfig()
	globalCfg.Seal()

	targetClient, err := raw.NewRPCClient(cfg.TargetChain.RPCAddr, cfg.TargetChain.Timeout, logger)
	if err != nil {
		logger.Fatal("could not initialize target rpc client", zap.Error(err))
	}

	neutronClient, err := raw.NewRPCClient(cfg.NeutronChain.RPCAddr, cfg.NeutronChain.Timeout, logger)
	if err != nil {
		logger.Fatal("cannot create neutron client", zap.Error(err))
	}

	connParams, err := loadConnParams(ctx, neutronClient, targetClient, cfg.NeutronChain.RESTAddr, cfg.NeutronChain.ConnectionID, logger)
	if err != nil {
		logger.Fatal("cannot load network params", zap.Error(err))
	}
	logger.Info("loaded conn params",
		zap.String("neutron_chain_id", connParams.neutronChainID),
		zap.String("target_chain_id", connParams.targetChainID),
		zap.String("neutron_client_id", connParams.neutronClientID),
		zap.String("target_client_id", connParams.targetClientID),
		zap.String("target_connection_id", connParams.targetConnectionID))

	targetQuerier, err := tmquerier.NewQuerier(targetClient, connParams.targetChainID, cfg.TargetChain.ValidatorAccountPrefix)
	if err != nil {
		logger.Fatal("cannot connect to target chain", zap.Error(err))
	}

	codec := raw.MakeCodecDefault()
	keybase, err := submit.TestKeybase(connParams.neutronChainID, cfg.NeutronChain.HomeDir)
	if err != nil {
		logger.Fatal("cannot initialize keybase", zap.Error(err))
	}

	txSender, err := submit.NewTxSender(ctx, neutronClient, codec.Marshaller, keybase, *cfg.NeutronChain, logger, connParams.neutronChainID)
	if err != nil {
		logger.Fatal("cannot create tx sender", zap.Error(err))
	}

	var store relay.Storage

	if cfg.AllowTxQueries && cfg.StoragePath == "" {
		logger.Fatal("RELAYER_DB_PATH must be set with RELAYER_ALLOW_TX_QUERIES=true")
	}

	if cfg.StoragePath != "" {
		store, err = storage.NewLevelDBStorage(cfg.StoragePath)
		if err != nil {
			logger.Fatal("couldn't initialize levelDB storage", zap.Error(err))
		}
	} else {
		store = storage.NewDummyStorage()
	}

	neutronChain, targetChain, err := loadChains(cfg, connParams, logger)
	if err != nil {
		logger.Error("failed to loadChains", zap.Error(err))
	}

	var (
		proofSubmitter       = submit.NewSubmitterImpl(txSender, cfg.AllowKVCallbacks, neutronChain.PathEnd.ClientID)
		txQuerier            = txquerier.NewTXQuerySrv(targetQuerier.Client)
		trustedHeaderFetcher = trusted_headers.NewTrustedHeaderFetcher(neutronChain, targetChain, logger)
		txProcessor          = txprocessor.NewTxProcessor(trustedHeaderFetcher, store, proofSubmitter, logger)
		kvProcessor          = kvprocessor.NewKVProcessor(
			targetQuerier,
			cfg.MinKvUpdatePeriod,
			logger,
			proofSubmitter,
			store,
			targetChain,
			neutronChain,
		)
		txSubmitChecker = txsubmitchecker.NewTxSubmitChecker(
			txProcessor.GetSubmitNotificationChannel(),
			store,
			neutronClient,
			logger,
			cfg.CheckSubmittedTxStatusDelay,
		)
		relayer = relay.NewRelayer(
			cfg,
			txQuerier,
			store,
			txProcessor,
			kvProcessor,
			txSubmitChecker,
			logger,
		)
	)
	return relayer
}

func loadChains(cfg config.NeutronQueryRelayerConfig, connParams *connectionParams, logger *zap.Logger) (neutronChain *cosmosrelayer.Chain, targetChain *cosmosrelayer.Chain, err error) {
	targetChain, err = relay.GetTargetChain(logger, cfg.TargetChain, connParams.targetChainID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load target chain from env: %w", err)
	}

	if err := targetChain.AddPath(connParams.targetClientID, connParams.targetConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to source chain: %w", err)
	}

	if err := targetChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	neutronChain, err = relay.GetNeutronChain(logger, cfg.NeutronChain, connParams.neutronChainID)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to load neutron chain from env: %w", err)
	}

	if err := neutronChain.AddPath(connParams.neutronClientID, cfg.NeutronChain.ConnectionID); err != nil {
		return nil, nil, fmt.Errorf("failed to AddPath to destination chain: %w", err)
	}

	if err := neutronChain.ChainProvider.Init(); err != nil {
		return nil, nil, fmt.Errorf("failed to Init source chain provider: %w", err)
	}

	return neutronChain, targetChain, nil
}

type connectionParams struct {
	neutronChainID  string
	neutronClientID string

	targetChainID      string
	targetClientID     string
	targetConnectionID string
}

func loadConnParams(ctx context.Context, neutronClient, targetClient *rpcclienthttp.HTTP, restAddress string, neutronConnectionId string, logger *zap.Logger) (*connectionParams, error) {
	restClient, err := raw.NewRESTClient(restAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get newRESTClient: %w", err)
	}

	targetStatus, err := targetClient.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target chain status: %w", err)
	}

	neutronStatus, err := neutronClient.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch neutron chain status: %w", err)
	}

	// NOTE: waiting for connection to be created
	var client *query.IbcCoreConnectionV1ConnectionOK
	if err := retry.Do(func() error {
		var err error

		client, err = restClient.Query.IbcCoreConnectionV1Connection(&query.IbcCoreConnectionV1ConnectionParams{
			ConnectionID: neutronConnectionId,
			Context:      ctx,
		})

		if err != nil {
			return err
		}

		if client.GetPayload().Connection.Counterparty.ConnectionID == "" {
			return fmt.Errorf("empty target connection ID")
		}

		if client.GetPayload().Connection.Counterparty.ClientID == "" {
			return fmt.Errorf("empty target client ID")
		}

		return nil
	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr, retry.OnRetry(func(n uint, err error) {
		logger.Info(
			"failed to query ibc connection info", zap.Error(err))
	})); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query ibc connection info: %w", err)
	}

	return &connectionParams{
		neutronChainID:     neutronStatus.NodeInfo.Network,
		targetChainID:      targetStatus.NodeInfo.Network,
		neutronClientID:    client.GetPayload().Connection.ClientID,
		targetClientID:     client.GetPayload().Connection.Counterparty.ClientID,
		targetConnectionID: client.GetPayload().Connection.Counterparty.ConnectionID,
	}, nil
}
