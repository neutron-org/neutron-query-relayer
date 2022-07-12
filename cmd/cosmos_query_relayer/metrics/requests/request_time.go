package requests

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	labelMethod = "method"
)

type RequestTime struct {
	mu     *sync.Mutex
	metric *prometheus.HistogramVec
	values map[proofHistData]TimeRecord
}

func NewRequestTime() *RequestTime {
	return &RequestTime{
		mu: &sync.Mutex{},
		metric: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "relayer",
			Name:        "request_duration",
			Help:        "request duration",
			ConstLabels: prometheus.Labels{},
			Buckets:     []float64{0.5, 1, 2, 3, 5, 10, 30},
		}, []string{labelMethod, labelType}),
		values: map[proofHistData]TimeRecord{},
	}
}

func (m *RequestTime) Register(registry *prometheus.Registry) {
	registry.MustRegister(m.metric)
}

func (m *RequestTime) SetToPrometheus() {
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
func (m *RequestTime) Add(data proofHistData, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var record TimeRecord
	if existRecord, ok := m.values[data]; ok {
		record = existRecord
	}

	// update record with new values
	record.Add(duration)
	m.values[data] = record
}

type TimeRecord struct {
	Vec []float64
}

func (d *TimeRecord) Add(duration float64) {
	d.Vec = append(d.Vec, duration)
}

func (m *RequestTime) AddSuccess(method string, duration float64) {
	m.Add(proofHistData{
		method:  method,
		reqType: typeSuccess,
	}, duration)
}

func (m *RequestTime) AddFailed(method string, duration float64) {
	m.Add(proofHistData{
		method:  method,
		reqType: typeSuccess,
	}, duration)
}
