package relay

import (
	"context"
	"fmt"

	"github.com/cosmos/relayer/v2/relayer"

	"github.com/neutron-org/neutron-query-relayer/internal/config"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"

	"go.uber.org/zap"
)

// TxHeight describes tendermint filter by tx.height that we use to get only actual txs
const TxHeight = "tx.height"

// Relayer is controller for the whole app:
// 1. takes events from Neutron chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the Neutron chain
type Relayer struct {
	cfg         config.NeutronQueryRelayerConfig
	logger      *zap.Logger
	storage     Storage
	kvProcessor KVProcessor
	targetChain *relayer.Chain
}

func NewRelayer(
	cfg config.NeutronQueryRelayerConfig,
	store Storage,
	kvProcessor KVProcessor,
	targetChain *relayer.Chain,
	logger *zap.Logger,
) *Relayer {
	return &Relayer{
		cfg:         cfg,
		logger:      logger,
		storage:     store,
		kvProcessor: kvProcessor,
		targetChain: targetChain,
	}
}

// Run starts the relaying process: subscribes on the incoming interchain query messages from the
// Neutron and performs the queries by interacting with the target chain and submitting them to
// the Neutron chain.
func (r *Relayer) Run(
	ctx context.Context,
	queriesTasksQueue <-chan neutrontypes.RegisteredQuery, // Input tasks come from this channel
	submittedTxsTasksQueue chan PendingSubmittedTxInfo, // Tasks for the TxSubmitChecker are sent to this channel
) error {
	for {
		var err error
		select {
		case query := <-queriesTasksQueue:
			switch query.QueryType {
			case string(neutrontypes.InterchainQueryTypeKV):
				msg := &MessageKV{QueryId: query.Id, KVKeys: query.Keys}
				err = r.processMessageKV(ctx, msg)
			default:
				err = fmt.Errorf("unknown query type: %s", query.QueryType)
			}

			if err != nil {
				r.logger.Error("could not process message", zap.Uint64("query_id", query.Id), zap.Error(err))
			}
		case <-ctx.Done():
			r.logger.Info("context cancelled, shutting down relayer...")
			return nil
		}
	}
}

// processMessageKV handles an incoming KV interchain query message and passes it to the kvProcessor for further processing.
func (r *Relayer) processMessageKV(ctx context.Context, m *MessageKV) error {
	r.logger.Debug("running processMessageKV for msg", zap.Uint64("query_id", m.QueryId))
	return r.kvProcessor.ProcessAndSubmit(ctx, m)
}
