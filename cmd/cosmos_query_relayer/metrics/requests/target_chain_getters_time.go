package requests

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type TargetChainGettersTime struct {
	mu     *sync.Mutex
	metric *prometheus.HistogramVec
	values map[proofHistData]TimeRecord
}

func NewProofTargetTime() *TargetChainGettersTime {
	return &TargetChainGettersTime{
		mu: &sync.Mutex{},
		metric: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "relayer",
			Name:        "proof_target_duration",
			Help:        "proof on target chain duration",
			ConstLabels: prometheus.Labels{},
			Buckets:     []float64{0.5, 1, 2, 3, 5, 10, 30},
		}, []string{labelMethod, labelType}),
		values: map[proofHistData]TimeRecord{},
	}
}

func (m *TargetChainGettersTime) Register(registry *prometheus.Registry) {
	registry.MustRegister(m.metric)
}

func (m *TargetChainGettersTime) SetToPrometheus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metric.Reset()
	for data, v := range m.values {
		for _, elem := range v.Vec {
			m.metric.With(prometheus.Labels{
				labelType:   data.reqType,
				labelMethod: data.method,
			}).Observe(elem)
		}
	}
	m.values = map[proofHistData]TimeRecord{}
}

// Add duration in seconds of message processing time
func (m *TargetChainGettersTime) Add(proofData proofHistData, duration float64) {
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

func (m *TargetChainGettersTime) AddSuccess(method string, duration float64) {
	m.Add(proofHistData{
		method:  method,
		reqType: typeSuccess,
	}, duration)
}

func (m *TargetChainGettersTime) AddFailed(method string, duration float64) {
	m.Add(proofHistData{
		method:  method,
		reqType: typeFailed,
	}, duration)
}
