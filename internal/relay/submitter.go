package relay

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
)

// Submitter knows how to submit proof to the chain
type Submitter interface {
	SubmitProof(ctx context.Context, height uint64, queryId uint64, proof []proof.StorageValue, updateClientMsg sdk.Msg) error
	SubmitTxProof(ctx context.Context, queryId uint64, proof []proof.TxValue) error
}