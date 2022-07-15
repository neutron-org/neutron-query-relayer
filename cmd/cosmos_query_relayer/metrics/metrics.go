package metrics

import (
	"github.com/lidofinance/cosmos-query-relayer/cmd/cosmos_query_relayer/metrics/requests"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	metricsRegistry = prometheus.NewRegistry()
)

type PromMetric interface {
	// SetToPrometheus is expected to bulk-update specific metrics.
	SetToPrometheus()
}

type Client struct {
	ProofCount             *requests.ProofCount
	RequestTime            *requests.RequestTime
	ProofNeutronChainTime  *requests.ProofNeutronTime
	TargetChainGettersTime *requests.TargetChainGettersTime
}

func New() Client {
	client := Client{
		requests.NewProofCount(),
		requests.NewRequestTime(),
		requests.NewNeutronTime(),
		requests.NewTargetGettersTime(),
	}

	return client
}

func (c Client) Metrics() []PromMetric {
	return []PromMetric{
		c.ProofNeutronChainTime,
		c.TargetChainGettersTime,
		c.ProofCount,
		c.RequestTime,
	}
}

func NewMetricsHandler(metrics []PromMetric) http.Handler {
	return newHandler(metricsRegistry, metrics)
}

func newHandler(registry *prometheus.Registry, metrics []PromMetric) http.Handler {
	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		for _, m := range metrics {
			m.SetToPrometheus()
		}
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(rsp, req)
	})
}

func init() {
	metricsRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	metricsRegistry.MustRegister(collectors.NewGoCollector())
}
