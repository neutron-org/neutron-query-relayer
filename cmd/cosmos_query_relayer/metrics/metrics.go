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
	relayerFailedProofs = promauto.NewCounter(prometheus.CounterOpts{
		Name: "relayer_failed_proofs",
		Help: "The total number of failed requests (counter)",
	})

	relayerSuccessProofs = promauto.NewCounter(prometheus.CounterOpts{
		Name: "relayer_succeed_proofs",
		Help: "The total number of succeed requests (counter)",
	})

	requestTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_time",
		Help:    "A histogram of requests duration",
		Buckets: []float64{0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})

	proofNeutronTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "proof_neutron_time",
		Help:    "A histogram of proofs duration",
		Buckets: []float64{0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})

	targetChainGettersTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "target_chain_getters_time",
		Help:    "A histogram of target chain getters duration",
		Buckets: []float64{0.5, 1, 2, 3, 5, 10, 30},
	}, []string{labelMethod, labelType})
)

func IncFailedProofs() {
	relayerFailedProofs.Inc()
}

func IncSuccessProofs() {
	relayerSuccessProofs.Inc()
}

func AddFailedRequest(message string, dur float64) {
	requestTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeFailed,
	}).Observe(dur)
}

func AddSuccessRequest(message string, dur float64) {
	requestTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeSuccess,
	}).Observe(dur)
}

func AddFailedProof(message string, dur float64) {
	proofNeutronTime.With(prometheus.Labels{
		labelMethod: message,
		labelType:   typeFailed,
	}).Observe(dur)
}

func AddSuccessProof(message string, dur float64) {
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
