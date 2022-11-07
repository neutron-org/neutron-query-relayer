package webserver

import (
	"encoding/json"
	"net/http"

	nlogger "github.com/neutron-org/neutron-logger"

	"go.uber.org/zap"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/gorilla/mux"
)

const (
	ServerContext           = "webserver"
	UnsuccessfulTxsResource = "/unsuccessful-txs"
)

type ResponseTest struct {
	Txs []string
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request)

func Router(logRegistry *nlogger.Registry, store relay.Storage) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc(UnsuccessfulTxsResource, UnsuccessfulTxs(logRegistry.Get(ServerContext), store))
	return router
}

func UnsuccessfulTxs(logger *zap.Logger, store relay.Storage) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := store.GetAllUnsuccessfulTxs()
		if err != nil {
			logger.Error("failed to execute GetAllUnsuccessfulTxs", zap.Error(err))
			http.Error(w, "Error processing request", http.StatusInternalServerError)
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
