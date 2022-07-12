package requests

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ProofLidoTime struct {
	mu     *sync.Mutex
	metric *prometheus.HistogramVec
	values map[proofHistData]TimeRecord
}

func NewProofLidoTime() *ProofLidoTime {
	return &ProofLidoTime{
		mu: &sync.Mutex{},
		metric: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "relayer",
			Name:        "proof_duration",
			Help:        "proof tx submitting on lido duration",
			ConstLabels: prometheus.Labels{},
			Buckets:     []float64{0.5, 1, 2, 3, 5, 10, 30},
		}, []string{labelMethod, labelType}),
		values: map[proofHistData]TimeRecord{},
	}
}

func (m *ProofLidoTime) Register(registry *prometheus.Registry) {
	registry.MustRegister(m.metric)
}

func (m *ProofLidoTime) SetToPrometheus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metric.Reset()
	for data, v := range m.values {
		for _, elem := range v.Vec {
			m.metric.With(prometheus.Labels{
				labelMethod: data.method,
				labelType:   data.reqType,
			}).Observe(elem)
		}
	}
	m.values = map[proofHistData]TimeRecord{}
}

// Add duration in seconds of message processing time
func (m *ProofLidoTime) Add(proofData proofHistData, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var record TimeRecord
	if existRecord, ok := m.values[proofData]; ok {
		record = existRecord
	}

	// update record with new values
	record.Add(duration)
	m.values[proofData] = record
}

func (m *ProofLidoTime) AddSuccess(method string, duration float64) {
	m.Add(proofHistData{
		method:  method,
		reqType: typeSuccess,
	}, duration)
}

func (m *ProofLidoTime) AddFailed(method string, duration float64) {
	m.Add(proofHistData{
		method:  method,
		reqType: typeFailed,
	}, duration)
}

type proofHistData struct {
	method  string
	reqType string
}
