package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/syndtr/goleveldb/leveldb"

	nlogger "github.com/neutron-org/neutron-logger"

	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/gorilla/mux"
)

const (
	ServerContext           = "http"
	UnsuccessfulTxsResource = "/unsuccessful-txs"
	ResubmitTxs             = "/resubmit-txs"
	PrometheusMetrics       = "/metrics"
)

type ResubmitTx struct {
	QueryID uint64 `json:"query_id"`
	Hash    string `json:"hash"`
}

type ResubmitRequest struct {
	Txs []ResubmitTx `json:"txs"`
}

func Run(ctx context.Context, logRegistry *nlogger.Registry, storage relay.Storage, txProcessor relay.TXProcessor, submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo, ListenAddr string) error {
	server := &http.Server{
		Addr:    ListenAddr,
		Handler: Router(logRegistry, storage, txProcessor, submittedTxsTasksQueue),
	}
	logger := logRegistry.Get(ServerContext)
	errch := make(chan error)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				logger.Error("failed to serve http", zap.Error(err))
				errch <- err
			}
		}
	}()

	select {
	case err := <-errch:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down the api http")
	webserverCtx, cancelWebserverCtx := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelWebserverCtx()
	if err := server.Shutdown(webserverCtx); err != nil {
		logger.Error("failed to shutdown api http gracefully: %w", zap.Error(err))
		return nil
	}

	logger.Info("api http shut down successfully")
	return nil
}

func Router(logRegistry *nlogger.Registry, storage relay.Storage, txProcessor relay.TXProcessor, submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo) *mux.Router {
	promHandler := NewPromWrapper(logRegistry, storage)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc(UnsuccessfulTxsResource, unsuccessfulTxs(logRegistry.Get(ServerContext), storage))
	router.HandleFunc(ResubmitTxs, resubmitFailedTxs(logRegistry.Get(ServerContext), storage, txProcessor, submittedTxsTasksQueue)).Methods(http.MethodPost)
	router.Handle(PrometheusMetrics, promHandler)
	return router
}

func unsuccessfulTxs(logger *zap.Logger, storage relay.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := storage.GetAllUnsuccessfulTxs()
		if err != nil {
			logger.Error("failed to execute GetAllUnsuccessfulTxs", zap.Error(err))
			http.Error(w, "Error processing request", http.StatusInternalServerError)
			return
		}

		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(res)
		if err != nil {
			logger.Error("failed to encode result of GetAllUnsuccessfulTxs", zap.Error(err))
			http.Error(w, "Error processing request", http.StatusInternalServerError)
		}
	}
}

func resubmitFailedTxs(logger *zap.Logger, store relay.Storage, txProcessor relay.TXProcessor, submittedTxsTasksQueue chan relay.PendingSubmittedTxInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqBody := ResubmitRequest{}
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&reqBody)
		if err != nil {
			logger.Error("failed to decode request body of resubmitFailedTxs", zap.Error(err))
			http.Error(w, fmt.Sprintf("Error processing request: %s", err), http.StatusInternalServerError)
			return
		}

		for _, txInfo := range reqBody.Txs {
			logger.Debug("resubmitting tx", zap.Uint64("query_id", txInfo.QueryID), zap.String("hash", txInfo.Hash))
			tx, err := store.GetCachedTx(txInfo.QueryID, txInfo.Hash)
			if err != nil {
				logger.Error("failed to get unsuccessful tx", zap.Error(err))
				httpErrorCode := http.StatusInternalServerError
				httpErrorMessage := fmt.Sprintf("Error processing request: %s", err)
				if errors.As(err, &leveldb.ErrNotFound) {
					httpErrorCode = http.StatusBadRequest
					httpErrorMessage = fmt.Sprintf("no tx found with queryID=%d and hash=%s", txInfo.QueryID, txInfo.Hash)
				}
				http.Error(w, httpErrorMessage, httpErrorCode)
				return
			}
			logger.Debug("tx", zap.Any("tx", *tx))
			// we do not want to pass r.Context() at this place, because r.Context() is canceled at the end of the function
			// but we have delayed call of txsubmitchecker which depends on the context passed into the ProcessAndSubmit
			err = txProcessor.ProcessAndSubmit(context.Background(), txInfo.QueryID, *tx, submittedTxsTasksQueue)
			if err != nil {
				logger.Error("failed to process and resubmit tx", zap.Error(err))
				http.Error(w, fmt.Sprintf("Error processing request: %s", err), http.StatusInternalServerError)
				return
			}
		}
	}
}
