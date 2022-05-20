package submit

import "github.com/lidofinance/cosmos-query-relayer/internal/proof"

type Submitter interface {
	SubmitProof(height uint64, queryId uint64, proof []proof.StorageValue) error
	SubmitTxProof(queryId uint64, proof []proof.TxValue) error
}
