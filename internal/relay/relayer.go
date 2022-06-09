package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	lidotypes "github.com/lidofinance/gaia-wasm-zone/x/interchainqueries/types"
	"strconv"

	"github.com/avast/retry-go/v4"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/cosmos/relayer/v2/relayer/provider/cosmos"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
)

// Relayer is controller for the whole app:
// 1. takes events from lido chain
// 2. dispatches each query by type to fetch proof for the right query
// 3. submits proof for a query back to the lido chain
type Relayer struct {
	proofer     Proofer
	submitter   Submitter
	targetChain *relayer.Chain
	lidoChain   *relayer.Chain

	targetChainId     string
	targetChainPrefix string
}

func NewRelayer(
	proofer Proofer,
	submitter Submitter,
	targetChainId,
	targetChainPrefix string,
	srcChain,
	dstChain *relayer.Chain,
) Relayer {
	return Relayer{
		proofer:           proofer,
		submitter:         submitter,
		targetChainId:     targetChainId,
		targetChainPrefix: targetChainPrefix,
		targetChain:       srcChain,
		lidoChain:         dstChain,
	}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) error {
	messages, err := r.tryExtractInterchainQueries(event)
	if err != nil {
		return fmt.Errorf("could not filter interchain query messages: %w", err)
	}

	for _, m := range messages {
		err := r.proofMessage(ctx, m)
		if err != nil {
			fmt.Printf("could not process message query_id=%d err=%s\n", m.queryId, err)
		} else {
			fmt.Printf("proof for query_id=%d submitted successfully\n", m.queryId)
		}
	}

	return nil
}

func (r Relayer) tryExtractInterchainQueries(event coretypes.ResultEvent) ([]queryEventMessage, error) {
	fmt.Printf("\nTry extracting events:\n%+v\n", event.Events)
	events := event.Events
	if len(events[zoneIdAttr]) == 0 {
		return nil, nil
	}

	if len(events[zoneIdAttr]) != len(events[parametersAttr]) ||
		len(events[zoneIdAttr]) != len(events[queryIdAttr]) ||
		len(events[zoneIdAttr]) != len(events[typeAttr]) {
		return nil, fmt.Errorf("cannot filter interchain query messages because events attributes length does not match for events=%v", events)
	}

	messages := make([]queryEventMessage, 0, len(events[zoneIdAttr]))

	for idx, zoneId := range events[zoneIdAttr] {
		if zoneId != r.targetChainId {
			continue
		}

		queryIdStr := events[queryIdAttr][idx]
		queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
		if err != nil {
			fmt.Printf("invalid query_id format (not an uint): %+v", queryId)
			continue
		}

		messageType := events[typeAttr][idx]
		parameters := events[parametersAttr][idx]

		messages = append(messages, queryEventMessage{queryId: queryId, messageType: messageType, parameters: []byte(parameters)})
	}

	return messages, nil
}

func (r Relayer) proofMessage(ctx context.Context, m queryEventMessage) error {
	fmt.Printf("proofMessage message_type=%s\n", m.messageType)

	latestHeight, err := r.lidoChain.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to QueryLatestHeight: %w", err)
	}

	updateClientMsg, err := r.getUpdateClientMsg(ctx, latestHeight)
	if err != nil {
		return fmt.Errorf("failed to getUpdateClientMsg: %w", err)
	}

	switch m.messageType {
	case delegatorDelegationsType:
		var params delegatorDelegationsParams
		err := json.Unmarshal(m.parameters, &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetDelegatorDelegations with params=%s query_id=%d: %w",
				m.parameters, m.queryId, err)
		}

		proofs, height, err := r.proofer.GetDelegatorDelegations(ctx, uint64(latestHeight), r.targetChainPrefix, params.Delegator)
		if err != nil {
			return fmt.Errorf("could not get proof for GetDelegatorDelegations with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(ctx, height, m.queryId, proofs, updateClientMsg)
		if err != nil {
			return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}
	case getBalanceType:
		var params getBalanceParams
		err := json.Unmarshal(m.parameters, &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetBalance with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		proofs, height, err := r.proofer.GetBalance(ctx, uint64(latestHeight), r.targetChainPrefix, params.Addr, params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(ctx, height, m.queryId, proofs, updateClientMsg)
		if err != nil {
			return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}
	case exchangeRateType:
		var params exchangeRateParams
		err := json.Unmarshal(m.parameters, &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for ExchangeRate with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		supplyProofs, height, err := r.proofer.GetSupply(ctx, uint64(latestHeight), params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for ExchangeRate with query_id=%d: %w", m.queryId, err)
		}

		delegationProofs, _, err := r.proofer.GetDelegatorDelegations(ctx, uint64(latestHeight), r.targetChainPrefix, params.Delegator)

		err = r.submitter.SubmitProof(ctx, height, m.queryId, append(supplyProofs, delegationProofs...), updateClientMsg)
		if err != nil {
			return fmt.Errorf("could not submit proof for %s with query_id=%d: %w", m.messageType, m.queryId, err)
		}
	case recipientTransactionsType:
		var params recipientTransactionsParams
		err := json.Unmarshal(m.parameters, &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for RecipientTransactions with params=%s query_id=%d: %w",
				m.parameters, m.queryId, err)
		}

		blocks, err := r.proofer.RecipientTransactions(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for RecipientTransactions with query_id=%d: %w", m.queryId, err)
		}

		resultBlocks := make([]*lidotypes.Block, 0, len(blocks))
		for height, txs := range blocks {
			header, err := r.getSrcChainHeader(ctx, int64(height))
			if err != nil {
				return fmt.Errorf("failed to get header for src chain: %w", err)
			}

			packedHeader, err := clienttypes.PackHeader(header)
			if err != nil {
				return fmt.Errorf("failed to pack header: %w", err)
			}

			nextHeader, err := r.getSrcChainHeader(ctx, int64(height+1))
			if err != nil {
				return fmt.Errorf("failed to get next header for src chain: %w", err)
			}

			packedNextHeader, err := clienttypes.PackHeader(nextHeader)
			if err != nil {
				return fmt.Errorf("failed to pack header: %w", err)
			}

			resultBlocks = append(resultBlocks, &lidotypes.Block{
				Header:          packedHeader,
				NextBlockHeader: packedNextHeader,
				Txs:             txs,
			})
		}

		err = r.submitter.SubmitTxProof(ctx, m.queryId, r.lidoChain.PathEnd.ClientID, resultBlocks)
		if err != nil {
			return fmt.Errorf("could not submit proof for RecipientTransactions with query_id=%d: %w", m.queryId, err)
		}

	case delegationRewardsType:
		return fmt.Errorf("could not relay not implemented query x/distribution/CalculateDelegationRewards")

	default:
		return fmt.Errorf("unknown query message type=%s", m.messageType)
	}

	return nil
}

func (r *Relayer) getSrcChainHeader(ctx context.Context, height int64) (ibcexported.Header, error) {
	var srcHeader ibcexported.Header
	if err := retry.Do(func() error {
		var err error
		srcHeader, err = r.targetChain.ChainProvider.GetIBCUpdateHeader(ctx, height, r.lidoChain.ChainProvider, r.lidoChain.PathEnd.ClientID)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		// TODO: this sometimes triggers the following error: failed to GetIBCUpdateHeader: height requested is too high,
		//		but eventually it goes away.
		fmt.Println(
			"failed to GetIBCUpdateHeader:", err,
		)
	})); err != nil {
		return nil, err
	}

	return srcHeader, nil
}

func (r *Relayer) getUpdateClientMsg(ctx context.Context, srch int64) (sdk.Msg, error) {
	// Query IBC Update Header
	srcHeader, err := r.getSrcChainHeader(ctx, srch)
	if err != nil {
		return nil, err
	}

	// Construct UpdateClient msg
	var updateMsgRelayer provider.RelayerMessage
	if err := retry.Do(func() error {
		var err error
		updateMsgRelayer, err = r.lidoChain.ChainProvider.UpdateClient(r.lidoChain.PathEnd.ClientID, srcHeader)
		return err
	}, retry.Context(ctx), relayer.RtyAtt, relayer.RtyDel, relayer.RtyErr, retry.OnRetry(func(n uint, err error) {
		fmt.Println(
			"failed to build message:", err,
		)
	})); err != nil {
		return nil, err
	}

	updateMsgUnpacked, ok := updateMsgRelayer.(cosmos.CosmosMessage)
	if !ok {
		return nil, errors.New("failed to cast provider.RelayerMessage to cosmos.CosmosMessage")
	}

	return updateMsgUnpacked.Msg, nil
}
