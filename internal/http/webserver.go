package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	nlogger "github.com/neutron-org/neutron-logger"

	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/gorilla/mux"
)

const (
	ServerContext           = "http"
	UnsuccessfulTxsResource = "/unsuccessful-txs"
	PrometheusMetrics       = "/metrics"
)

func Run(ctx context.Context, logRegistry *nlogger.Registry, storage relay.Storage, ListenAddr string) error {
	server := &http.Server{
		Addr:    ListenAddr,
		Handler: Router(logRegistry, storage),
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

func Router(logRegistry *nlogger.Registry, storage relay.Storage) *mux.Router {
	promHandler := NewPromWrapper(logRegistry, storage)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc(UnsuccessfulTxsResource, unsuccessfulTxs(logRegistry.Get(ServerContext), storage))
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
