package proof_impl

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/neutron-org/cosmos-query-relayer/internal/proof"
	"github.com/neutron-org/cosmos-query-relayer/internal/raw"
)

// GetDelegatorDelegations gets proofs for query type = 'x/staking/GetDelegatorDelegations'
func (p ProoferImpl) GetDelegatorDelegations(ctx context.Context, inputHeight uint64, prefix string, delegator string) ([]proof.StorageValue, uint64, error) {
	storeKey := stakingtypes.StoreKey
	delegatorBz, err := sdk.GetFromBech32(delegator, prefix)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode address from bech32: %w", err)
	}

	delegatorPrefixKey := stakingtypes.GetDelegationsKey(delegatorBz)

	result, height, err := p.querier.QueryIterateTendermintProof(ctx, int64(inputHeight), storeKey, delegatorPrefixKey)

	validatorsWithProofs := make([]proof.StorageValue, 0, len(result))
	for _, delegationBz := range result {
		delegation, err := stakingtypes.UnmarshalDelegation(raw.MakeCodecDefault().Marshaller, delegationBz.Value)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal delegation: %w", err)
		}

		valAddr, err := sdk.GetFromBech32(delegation.ValidatorAddress, p.querier.ValidatorAccountPrefix)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to decode validator address from bech32: %w", err)
		}

		valKey := stakingtypes.GetValidatorKey(valAddr)

		validatorResult, _, err := p.querier.QueryTendermintProof(ctx, int64(inputHeight), storeKey, valKey)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get validator's raw value from storage with proofs: %w", err)
		}

		validatorsWithProofs = append(validatorsWithProofs, *validatorResult)
	}

	return append(result, validatorsWithProofs...), height, err
}
