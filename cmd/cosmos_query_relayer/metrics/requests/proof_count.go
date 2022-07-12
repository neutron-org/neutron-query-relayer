package requests

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	labelType = "type"
)

const (
	typeSuccess = "success"
	typeFailed  = "failed"
)

type ProofCount struct {
	mu     *sync.Mutex
	metric *prometheus.GaugeVec
	values map[proofCountData]int
}

func NewProofCount() *ProofCount {
	return &ProofCount{
		mu: &sync.Mutex{},
		metric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "proof_count",
			Help: "Count of http requests",
		}, []string{labelType}),
		values: map[proofCountData]int{},
	}
}

func (m *ProofCount) Register(registry *prometheus.Registry) {
	registry.MustRegister(m.metric)
}

func (m *ProofCount) SetToPrometheus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metric.Reset()
	for data, value := range m.values {
		m.metric.With(prometheus.Labels{
			labelType: data.metricType,
		}).Set(float64(value))
	}
	m.values = map[proofCountData]int{}
}

func (m *ProofCount) add(data proofCountData) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.values[data]++
}

func (m *ProofCount) AddSucceess() {
	m.add(proofCountData{
		metricType: typeSuccess,
	})
}

func (m *ProofCount) AddFailed() {
	m.add(proofCountData{
		metricType: typeFailed,
	})
}

type proofCountData struct {
	metricType string
}
