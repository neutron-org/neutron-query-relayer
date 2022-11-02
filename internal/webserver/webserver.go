package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/neutron-org/neutron-query-relayer/internal/relay"

	"github.com/gorilla/mux"
)

type ResponseTest struct {
	Txs []string
}

func Router(store relay.Storage) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/unsuccessful_txs", UnsuccessfulTxs(store))
	return router
}

func UnsuccessfulTxs(store relay.Storage) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		res := ResponseTest{
			Txs: []string{"kekw", "lulz"},
		}

		err := json.NewEncoder(w).Encode(res)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
	}
}
