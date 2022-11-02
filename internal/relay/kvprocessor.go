package relay

import "context"

// KVProcessor processes event query KV type. Obtains the proof for a query we need to process, and sends it to  the neutron
type KVProcessor interface {
	// ProcessAndSubmit handles an incoming KV interchain query message. It checks whether it's time
	// to execute the query (based on the relayer's settings), queries values and proofs for the query
	// keys, and submits the result to the Neutron chain.
	ProcessAndSubmit(ctx context.Context, m *MessageKV) error
}
