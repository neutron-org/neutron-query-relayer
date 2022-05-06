package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// TODO
func ProofRewards(ctx context.Context, querier *proofer.ProofQuerier, prefix, validatorAddressBech32, delegatorAddressBech32 string, endingPeriod uint64) error {
	// Get latest block for latest height
	block, err := querier.Client.Block(ctx, nil)
	if err != nil {
		return fmt.Errorf("error fetching latest block: %w", err)
	}

	// Getting starting info
	validatorAddressBytes, err := cosmostypes.GetFromBech32(validatorAddressBech32, prefix+cosmostypes.PrefixValidator+cosmostypes.PrefixOperator)
	err = cosmostypes.VerifyAddressFormat(validatorAddressBytes)
	if err != nil {
		return fmt.Errorf("error converting validator address from bech32: %w", err)
	}
	delegatorAddressBytes, err := cosmostypes.GetFromBech32(delegatorAddressBech32, prefix)
	if err != nil {
		return fmt.Errorf("error converting delegator address from bech32: %w", err)
	}
	startingInfoKey := distributiontypes.GetDelegatorStartingInfoKey(validatorAddressBytes, delegatorAddressBytes)
	height := int64(0) // TODO: height?
	startingInfoStorageValue, err := querier.QueryTendermintProof(ctx, height, distributiontypes.StoreKey, startingInfoKey)
	if err != nil {
		return fmt.Errorf("error fetching proof for starting info: %w", err)
	}

	var startingInfo distributiontypes.DelegatorStartingInfo
	err = startingInfo.Unmarshal(startingInfoStorageValue.Value)
	if err != nil {
		return fmt.Errorf("error unmarshalling starting info: %w", err)
	}
	fmt.Printf("Starting info: %+v\n", startingInfo)

	// TODO: get delegation shares proof? or is it proven in other types?

	startingHeight := startingInfo.Height
	endingHeight := uint64(block.Block.Height)
	_ = distributiontypes.GetValidatorSlashEventKeyPrefix(validatorAddressBytes, startingHeight) // _fromPrefix
	_ = distributiontypes.GetValidatorSlashEventKeyPrefix(validatorAddressBytes, endingHeight+1) // toPrefix

	p := distributiontypes.GetValidatorSlashEventPrefix(validatorAddressBytes)
	storageValue, err := querier.QueryIterateTendermintProof(ctx, height, distributiontypes.StoreKey, p)
	fmt.Printf("Proof iterate: %+v", storageValue)
	//err = querier.Test(ctx, validatorAddressBytes, startingHeight, endingHeight)

	return nil
}
