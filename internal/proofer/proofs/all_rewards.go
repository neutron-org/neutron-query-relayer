package proofs

import (
	"context"
	"fmt"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

// TODO
// See cosmos-sdk x/distribution/keeper/delegation.go #CalculateDelegationRewards
func ProofRewards(ctx context.Context, querier *proofer.ProofQuerier, prefix, validatorAddressBech32, delegatorAddressBech32 string, endingPeriod uint64) error {
	// Get latest block for latest height
	block, err := querier.Client.Block(ctx, nil)
	if err != nil {
		return fmt.Errorf("error fetching latest block: %w", err)
	}

	// Getting starting info
	validatorAddressBz, err := cosmostypes.GetFromBech32(validatorAddressBech32, prefix+cosmostypes.PrefixValidator+cosmostypes.PrefixOperator)
	err = cosmostypes.VerifyAddressFormat(validatorAddressBz)
	if err != nil {
		return fmt.Errorf("error converting validator address from bech32: %w", err)
	}
	delegatorAddressBz, err := cosmostypes.GetFromBech32(delegatorAddressBech32, prefix)
	if err != nil {
		return fmt.Errorf("error converting delegator address from bech32: %w", err)
	}
	startingInfoKey := distributiontypes.GetDelegatorStartingInfoKey(validatorAddressBz, delegatorAddressBz)
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
	_ = distributiontypes.GetValidatorSlashEventKeyPrefix(validatorAddressBz, startingHeight) // _fromPrefix
	_ = distributiontypes.GetValidatorSlashEventKeyPrefix(validatorAddressBz, endingHeight+1) // toPrefix

	//p := distributiontypes.GetValidatorSlashEventPrefix(validatorAddressBz)
	//storageValue, err := querier.QueryIterateTendermintProof(ctx, height, distributiontypes.S	toreKey, p)
	//fmt.Printf("Proof iterate: %+v", storageValue)
	//err = querier.Test(ctx, validatorAddressBz, startingHeight, endingHeight)

	// TODO: filter out slashes with height more than needed
	allSlashes, err := querier.QueryIterateTendermintProof(ctx, height, distributiontypes.StoreKey, distributiontypes.GetValidatorSlashEventKeyPrefix(validatorAddressBz, startingHeight))
	if err != nil {
		return fmt.Errorf("error querying proofs for slashes: %w", err)
	}

	// TODO: check that we're not missing any periods, starting with `startingInfo.PreviousPeriod` and ending with `WHAT`?
	// TODO: what is a period?
	// Collect periods to calculate rewards from
	rewardPeriods := make([]uint64, 0, len(allSlashes)+2)
	rewardPeriods = append(rewardPeriods, startingInfo.PreviousPeriod)
	for _, item := range allSlashes {
		var event distributiontypes.ValidatorSlashEvent
		err = event.Unmarshal(item.Value)
		if err != nil {
			return fmt.Errorf("error unmarshalling ValidatorSlashEvent: %w", err)
		}

		rewardPeriods = append(rewardPeriods, event.ValidatorPeriod)
	}
	fmt.Printf("All slashes: %+v\n", allSlashes)

	// For every needed period look for rewards
	for _, item := range rewardPeriods {
		value, err := querier.QueryTendermintProof(ctx, height, distributiontypes.StoreKey, distributiontypes.GetValidatorHistoricalRewardsKey(validatorAddressBz, item))
		if err != nil {
			return fmt.Errorf("could not query reward tendermint proof for period=%d: %w", item, err)
		}

		var rewards distributiontypes.ValidatorHistoricalRewards
		err = rewards.Unmarshal(value.Value)
		if err != nil {
			return fmt.Errorf("could not unmarshal rewards for period=%d: %w", item, err)
		}
		fmt.Printf("Rewards for period=%d: %+v\n", item, rewards)
	}

	return nil
}
