package relay

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	neutrontypes "github.com/neutron-org/neutron/x/interchainqueries/types"
)

// Submitter knows how to submit proof to the chain
type Submitter interface {
	SubmitKVProof(height, revision, queryId uint64, proof []*neutrontypes.StorageValue, updateClientMsg sdk.Msg) error
	SubmitTxProof(queryId uint64, proof *neutrontypes.Block) (string, error)
}
