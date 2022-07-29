package relay

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Submitter knows how to submit proof to the chain
type Submitter interface {
	SubmitProof(ctx context.Context, height, revision, queryId uint64, allowKVCallbacks bool, proof []proof.StorageValue, updateClientMsg sdk.Msg) error
	SubmitTxProof(ctx context.Context, queryId uint64, clientID string, proof *neutrontypes.Block) error
}
