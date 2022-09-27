package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb"

	neutronmetrics "github.com/neutron-org/neutron-query-relayer/cmd/neutron_query_relayer/metrics"
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
	cfg             config.NeutronQueryRelayerConfig
	txQuerier       TXQuerier
	logger          *zap.Logger
	storage         Storage
	txProcessor     TXProcessor
	txSubmitChecker TxSubmitChecker
	kvProcessor     KVProcessor
}

func NewRelayer(
	cfg config.NeutronQueryRelayerConfig,
	txQuerier TXQuerier,
	store Storage,
	txProcessor TXProcessor,
	kvprocessor KVProcessor,
	txSubmitChecker TxSubmitChecker,
	logger *zap.Logger,
) *Relayer {
	return &Relayer{
		cfg:             cfg,
		txQuerier:       txQuerier,
		logger:          logger,
		storage:         store,
		txProcessor:     txProcessor,
		txSubmitChecker: txSubmitChecker,
		kvProcessor:     kvprocessor,
	}
}

// Run starts the relaying process: subscribes on the incoming interchain query messages from the
// Neutron and performs the queries by interacting with the target chain and submitting them to
// the Neutron chain.
func (r *Relayer) Run(ctx context.Context, tasks <-chan neutrontypes.RegisteredQuery) error {
	err := r.txSubmitChecker.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize tx submit checker: %w", err)
	}

	r.logger.Info("successfully initialized tx submit checker")

	for {
		var (
			start     time.Time
			queryType neutrontypes.InterchainQueryType
			queryID   uint64
			err       error
		)
		select {
		case query := <-tasks:
			switch query.QueryType {
			case string(neutrontypes.InterchainQueryTypeKV):
				msg := &MessageKV{QueryId: query.Id, KVKeys: query.Keys}
				err = r.processMessageKV(ctx, msg)
			case string(neutrontypes.InterchainQueryTypeTX):
				msg := &MessageTX{QueryId: query.Id, TransactionsFilter: query.TransactionsFilter}
				err = r.processMessageTX(ctx, msg)
			default:
				err = fmt.Errorf("unknown query type: %s", query.QueryType)
			}

			if err != nil {
				r.logger.Error("could not process message", zap.Uint64("query_id", queryID), zap.Error(err))
				neutronmetrics.AddFailedRequest(string(queryType), time.Since(start).Seconds())
			} else {
				neutronmetrics.AddSuccessRequest(string(queryType), time.Since(start).Seconds())
			}
		case <-ctx.Done():
			r.logger.Info("Context cancelled, shittung down relayer...")
			return r.stop()
		}
	}
}

// stop finishes execution of relayer's auxiliary entities.
func (r *Relayer) stop() error {
	var failed bool
	if err := r.storage.Close(); err != nil {
		r.logger.Error("failed to close relayer's storage", zap.Error(err))
		failed = true
	} else {
		r.logger.Info("relayer's storage has been closed")
	}

	if failed {
		return fmt.Errorf("error occurred while stopping relayer, see recent logs for more info")
	}

	return nil
}

// processMessageKV handles an incoming KV interchain query message and passes it to the kvProcessor for further processing.
func (r *Relayer) processMessageKV(ctx context.Context, m *MessageKV) error {
	r.logger.Debug("running processMessageKV for msg", zap.Uint64("query_id", m.QueryId))
	return r.kvProcessor.ProcessAndSubmit(ctx, m)
}

func (r *Relayer) buildTxQuery(m *MessageTX) (string, error) {
	queryLastHeight, err := r.getLastQueryHeight(m.QueryId)
	if err != nil {
		return "", fmt.Errorf("could not get last query height: %w", err)
	}

	var params neutrontypes.TransactionsFilter
	if err = json.Unmarshal([]byte(m.TransactionsFilter), &params); err != nil {
		return "", fmt.Errorf("could not unmarshal transactions filter: %w", err)
	}
	// add filter by tx.height (tx.height>n)
	params = append(params, neutrontypes.TransactionsFilterItem{Field: TxHeight, Op: "gt", Value: queryLastHeight})

	queryString, err := queryFromTxFilter(params)
	if err != nil {
		return "", fmt.Errorf("failed to process tx query params: %w", err)
	}

	return queryString, nil
}

// processMessageTX handles an incoming TX interchain query message. It fetches proven transactions
// from the target chain using the message transactions filter value, and submits the result to the
// Neutron chain.
func (r *Relayer) processMessageTX(ctx context.Context, m *MessageTX) error {
	r.logger.Debug("running processMessageTX for msg", zap.Uint64("query_id", m.QueryId))
	queryString, err := r.buildTxQuery(m)
	if err != nil {
		return fmt.Errorf("failed to build tx query string: %w", err)
	}
	r.logger.Debug("tx query to search transactions", zap.Uint64("query_id", m.QueryId), zap.String("query", queryString))

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	txs, errs := r.txQuerier.SearchTransactions(cancelCtx, queryString)
	lastProcessedHeight := uint64(0)
	for tx := range txs {
		if tx.Height > lastProcessedHeight && lastProcessedHeight > 0 {
			err := r.storage.SetLastQueryHeight(m.QueryId, lastProcessedHeight)
			if err != nil {
				// TODO: should we stop after a first error
				return fmt.Errorf("failed to save last height of query: %w", err)
			}
			r.logger.Debug("block completely processed", zap.Uint64("query_id", m.QueryId), zap.Uint64("processed_height", lastProcessedHeight), zap.Uint64("next_height_to_process", tx.Height))
		}
		lastProcessedHeight = tx.Height
		err := r.txProcessor.ProcessAndSubmit(ctx, m.QueryId, tx)
		if err != nil {
			// TODO: should we stop after a first error
			return fmt.Errorf("failed to process txs: %w", err)
		}
	}

	stoppedWithErr := <-errs
	if stoppedWithErr != nil {
		return fmt.Errorf("failed to query txs: %w", stoppedWithErr)
	}

	if lastProcessedHeight > 0 {
		err = r.storage.SetLastQueryHeight(m.QueryId, lastProcessedHeight)
		if err != nil {
			return fmt.Errorf("failed to save last height of query: %w", err)
		}
		r.logger.Debug("the final block completely processed", zap.Uint64("query_id", m.QueryId), zap.Uint64("processed_height", lastProcessedHeight))
	} else {
		r.logger.Debug("no results found for the query", zap.Uint64("query_id", m.QueryId))
	}
	return nil
}

// getLastQueryHeight returns last query height & no err if query exists in storage, also initializes query with height = 0  if not exists yet
func (r *Relayer) getLastQueryHeight(queryID uint64) (uint64, error) {
	height, err := r.storage.GetLastQueryHeight(queryID)
	if err == leveldb.ErrNotFound {
		err = r.storage.SetLastQueryHeight(queryID, 0)
		if err != nil {
			return 0, fmt.Errorf("failed to set a 0 last height for an unitilialised query: %w", err)
		}
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to check query in storage: %w", err)
	}
	return height, nil
}

// QueryFromTxFilter creates query from transactions filter like
// `key1{=,>,>=,<,<=}value1 AND key2{=,>,>=,<,<=}value2 AND ...`
func queryFromTxFilter(params neutrontypes.TransactionsFilter) (string, error) {
	queryParamsList := make([]string, 0, len(params))
	for _, row := range params {
		sign, err := getOpSign(row.Op)
		if err != nil {
			return "", err
		}
		switch r := row.Value.(type) {
		case string:
			queryParamsList = append(queryParamsList, fmt.Sprintf("%s%s'%s'", row.Field, sign, r))
		case float64:
			queryParamsList = append(queryParamsList, fmt.Sprintf("%s%s%d", row.Field, sign, uint64(r)))
		case uint64:
			queryParamsList = append(queryParamsList, fmt.Sprintf("%s%s%d", row.Field, sign, r))
		}
	}
	return strings.Join(queryParamsList, " AND "), nil
}

func getOpSign(op string) (string, error) {
	switch strings.ToLower(op) {
	case "eq":
		return "=", nil
	case "gt":
		return ">", nil
	case "gte":
		return ">=", nil
	case "lt":
		return "<", nil
	case "lte":
		return "<=", nil
	default:
		return "", fmt.Errorf("unsupported operator %s", op)
	}
}
