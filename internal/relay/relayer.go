package relay

import (
	"context"
	"encoding/json"
	"fmt"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"strconv"
)

// Relayer is controller for the whole app:
// 1. takes events from lido chain
// 2. dispatches queries by type to fetch proof for the right query
// 3. submits proof for a query back to the lido chain
type Relayer struct {
	proofer           Proofer
	submitter         Submitter
	targetChainId     string
	targetChainPrefix string
	sender            string
}

func NewRelayer(proofer Proofer, submitter Submitter, targetChainId, targetChainPrefix string, sender string) Relayer {
	return Relayer{proofer: proofer, submitter: submitter, targetChainId: targetChainId, targetChainPrefix: targetChainPrefix, sender: sender}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) {
	messages, err := r.tryExtractInterchainQueries(event)
	if err != nil {
		fmt.Printf("could not filter intechain query messages: %s\n", err)
	}

	for _, m := range messages {
		err := r.proofMessage(ctx, m)
		if err != nil {
			fmt.Printf("could not process message query_id=%d err=%s\n", m.queryId, err)
		} else {
			fmt.Printf("proof for query_id=%d submitted successfully\n", m.queryId)
		}
	}
}

type eventValue []string

func (r Relayer) tryExtractInterchainQueries(event coretypes.ResultEvent) ([]queryEventMessage, error) {
	fmt.Printf("\n\nEvents: %+v\n\n", event.Events)
	events := event.Events
	abciMessages := make(map[string]eventValue, 0)

	if len(events[queryIdAttr]) == 0 {
		return []queryEventMessage{}, nil
	}

	if len(events[zoneIdAttr]) != len(events[parametersAttr]) ||
		len(events[zoneIdAttr]) != len(events[queryIdAttr]) ||
		len(events[zoneIdAttr]) != len(events[typeAttr]) {
		return nil, fmt.Errorf("cannot filter interchain query messages because events attributes length does not match for events=%v", events)
	}

	messages := make([]queryEventMessage, 0, len(abciMessages))

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

		messages = append(messages,
			queryEventMessage{queryId: queryId, messageType: messageType, parameters: parameters})
	}

	return messages, nil
}

func (r Relayer) proofMessage(ctx context.Context, m queryEventMessage) error {
	fmt.Printf("proofMessage message_type=%s\n", m.messageType)
	switch m.messageType {
	case delegatorDelegationsType:
		var params delegatorDelegationsParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetDelegatorDelegations with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		proofs, height, err := r.proofer.GetDelegatorDelegations(ctx, 0, r.targetChainPrefix, params.Delegator)
		if err != nil {
			return fmt.Errorf("could not get proof for GetDelegatorDelegations with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(ctx, height, m.queryId, proofs)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetDelegatorDelegations with query_id=%d: %w", m.queryId, err)
		}
	case getBalanceType:
		var params getBalanceParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetBalance with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		proofs, height, err := r.proofer.GetBalance(ctx, 0, r.targetChainPrefix, params.Addr, params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(ctx, height, m.queryId, proofs)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}
	case exchangeRateType:
		var params exchangeRateParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for ExchangeRate with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		supplyProofs, supplyHeight, err := r.proofer.GetSupply(ctx, 0, params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for ExchangeRate with query_id=%d: %w", m.queryId, err)
		}

		delegationProofs, delegationHeight, err := r.proofer.GetDelegatorDelegations(ctx, supplyHeight, r.targetChainPrefix, params.Delegator)

		if delegationHeight != supplyHeight {
			return fmt.Errorf("heights for two parts of x/bank/ExchangeRate query does not match: delegationHeight=%d supplyHeight=%d", delegationHeight, supplyHeight)
		}

		err = r.submitter.SubmitProof(ctx, delegationHeight, m.queryId, append(supplyProofs, delegationProofs...))
		if err != nil {
			return fmt.Errorf("could not submit proof for ExchangeRate with query_id=%d: %w", m.queryId, err)
		}
	case recipientTransactionsType:
		var params recipientTransactionsParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for RecipientTransactions with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		txProof, err := r.proofer.RecipientTransactions(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for RecipientTransactions with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitTxProof(ctx, m.queryId, txProof)
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
