package webserver

import (
	"github.com/neutron-org/neutron-query-relayer/internal/metrics"
	"net/http"

	nlogger "github.com/neutron-org/neutron-logger"
	"github.com/neutron-org/neutron-query-relayer/internal/relay"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.uber.org/zap"
)

const MonitoringLoggerContext = "monitoring"

type PromWrapper struct {
	promHandler http.Handler
	storage     relay.Storage
	logger      *zap.Logger
}

func NewPromWrapper(logRegistry *nlogger.Registry, storage relay.Storage) PromWrapper {
	return PromWrapper{
		promHandler: promhttp.Handler(),
		storage:     storage,
		logger:      logRegistry.Get(MonitoringLoggerContext),
	}
}

func (p PromWrapper) fillUnsuccessfulTxsMetric() {
	txs, err := p.storage.GetAllUnsuccessfulTxs()
	if err != nil {
		p.logger.Error("failed to get unsuccessful txs from storage", zap.Error(err))
	}
	metrics.SetUnsuccessfulTxsSizeQueue(len(txs))
}

func (p PromWrapper) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	p.fillUnsuccessfulTxsMetric()
	p.promHandler.ServeHTTP(res, req)
}
