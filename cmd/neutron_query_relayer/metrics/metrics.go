package instrumenters

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

	targetChainGettersTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "target_chain_getters_time",
		Help:    "A histogram of target chain getters duration",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})

	submittedTxCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "submitted_txs",
		Help: "The total number of submitted txs (counter)",
	}, []string{labelType})
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

func AddFailedTargetChainGetter(message string, dur float64) {
	targetChainGettersTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeFailed,
	}).Observe(dur)
}

func AddSuccessTargetChainGetter(message string, dur float64) {
	targetChainGettersTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeSuccess,
	}).Observe(dur)
}

func IncSuccessTxSubmit() {
	relayerRequests.With(prometheus.Labels{
		labelType: typeSuccess,
	}).Inc()
}

func IncFailedTxSubmit() {
	relayerProofs.With(prometheus.Labels{
		labelType: typeFailed,
	}).Inc()
}
