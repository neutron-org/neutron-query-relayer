package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	labelMethod = "method"
	labelType   = "type"
	typeSuccess = "success"
	typeFailed  = "failed"
)

var (
	relayerRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "relayer_requests",
		Help: "The total number of requests (counter)",
	}, []string{labelType})

	relayerProofs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "relayer_proofs",
		Help: "The total number of proofs (counter)",
	}, []string{labelType})

	requestTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_time",
		Help:    "A histogram of requests duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})

	proofNeutronTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "proof_neutron_time",
		Help:    "A histogram of proofs duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})

	actionDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "action_durations",
		Help:    "A histogram of target chain getters duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})

	submittedTxCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "submitted_txs",
		Help: "The total number of submitted txs (counter)",
	}, []string{labelType})

	unsuccessfulTxsQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "unsuccessful_txs",
		Help: "The total number of unsuccessful txs in the storage",
	})

	subscriberTaskQueueNumElements = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "subscriber_task_queue_num_elements",
		Help: "The total number of elements in Subscriber's task queue",
	}, []string{})

	queriesToProcess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "queries_to_process",
		Help: "The total number of active registered queries to process (counter)",
	}, []string{})
)

func incFailedRequests() {
	relayerRequests.With(prometheus.Labels{
		labelType: typeFailed,
	}).Inc()
}

func incSuccessRequests() {
	relayerRequests.With(prometheus.Labels{
		labelType: typeSuccess,
	}).Inc()
}

func incFailedProofs() {
	relayerProofs.With(prometheus.Labels{
		labelType: typeFailed,
	}).Inc()
}

func incSuccessProofs() {
	relayerProofs.With(prometheus.Labels{
		labelType: typeSuccess,
	}).Inc()
}

func AddFailedRequest(message string, dur float64) {
	incFailedRequests()
	requestTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeFailed,
	}).Observe(dur)
}

func AddSuccessRequest(message string, dur float64) {
	incSuccessRequests()
	requestTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeSuccess,
	}).Observe(dur)
}

func AddFailedProof(message string, dur float64) {
	incFailedProofs()
	proofNeutronTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeFailed,
	}).Observe(dur)
}

func AddSuccessProof(message string, dur float64) {
	incSuccessProofs()
	proofNeutronTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeSuccess,
	}).Observe(dur)
}

func RecordActionDuration(action string, dur float64) {
	actionDurations.With(prometheus.Labels{
		labelMethod: action,
		labelType:   typeSuccess,
	}).Observe(dur)
}

func IncSuccessTxSubmit() {
	submittedTxCounter.With(prometheus.Labels{
		labelType: typeSuccess,
	}).Inc()
}

func IncFailedTxSubmit() {
	submittedTxCounter.With(prometheus.Labels{
		labelType: typeFailed,
	}).Inc()
}

func SetUnsuccessfulTxsSizeQueue(size int) {
	unsuccessfulTxsQueueSize.Set(float64(size))
}

func SetSubscriberTaskQueueNumElements(numElements int) {
	subscriberTaskQueueNumElements.With(prometheus.Labels{}).Set(float64(numElements))
}

func SetQueriesToProcessNumElements(numElements int) {
	queriesToProcess.With(prometheus.Labels{}).Set(float64(numElements))
}
