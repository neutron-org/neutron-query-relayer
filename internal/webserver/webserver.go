package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	nlogger "github.com/neutron-org/neutron-logger"

	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/gorilla/mux"
)

const (
	ServerContext           = "webserver"
	UnsuccessfulTxsResource = "/unsuccessful-txs"
	PrometheusMetrics       = "/metrics"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request)

func Run(ctx context.Context, logRegistry *nlogger.Registry, storage relay.Storage, WebServerPort int) error {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", WebServerPort),
		Handler: Router(logRegistry, storage),
	}
	logger := logRegistry.Get(ServerContext)
	errch := make(chan error)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				logger.Error("failed to serve webserver", zap.Error(err))
			}
			errch <- err
		}
	}()

	select {
	case err := <-errch:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down the api webserver")
	webserverCtx, cancelWebserverCtx := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelWebserverCtx()
	if err := server.Shutdown(webserverCtx); err != nil {
		logger.Error("failed to shutdown api webserver gracefully: %w", zap.Error(err))
		return nil
	}

	logger.Info("api webserver shut down successfully")
	return nil
}

func Router(logRegistry *nlogger.Registry, storage relay.Storage) *mux.Router {
	promHandler := NewPromWrapper(logRegistry, storage)
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc(UnsuccessfulTxsResource, UnsuccessfulTxs(logRegistry.Get(ServerContext), storage))
	router.Handle(PrometheusMetrics, promHandler)
	return router
}

func UnsuccessfulTxs(logger *zap.Logger, storage relay.Storage) HandlerFunc {
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
