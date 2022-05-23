package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"github.com/lidofinance/cosmos-query-relayer/internal/submit"
	lidotypes "github.com/lidofinance/gaia-wasm-zone/x/interchainqueries/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"strconv"
)

type Relayer struct {
	proofer           proof.Proofer
	submitter         submit.Submitter
	targetChainPrefix string
	sender            string
}

const (
	moduleAttr     = "message.module"
	queryIdAttr    = "message.query_id"
	parametersAttr = "message.parameters"
	typeAttr       = "message.type"
)

func NewRelayer(proofer proof.Proofer, submitter submit.Submitter, targetChainPrefix string, sender string) Relayer {
	return Relayer{proofer: proofer, submitter: submitter, targetChainPrefix: targetChainPrefix, sender: sender}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) {
	messages, err := tryExtractInterchainQueries(event)
	if err != nil {
		fmt.Printf("could not filter intechain query messages: %s", err)
	}

	for _, m := range messages {
		err := r.proofMessage(ctx, m)
		if err != nil {
			fmt.Printf("\ncould not process message query_id=%d err=%s\n", m.queryId, err)
		}
	}
}

type eventValue []string

func tryExtractInterchainQueries(event coretypes.ResultEvent) ([]queryEventMessage, error) {
	fmt.Printf("\n\nEvents: %+v\n\n", event.Events)
	events := event.Events
	abciMessages := make(map[string]eventValue, 0)

	if len(events[queryIdAttr]) == 0 {
		return []queryEventMessage{}, nil
	}

	if len(events[moduleAttr]) != len(events[parametersAttr]) ||
		len(events[moduleAttr]) != len(events[queryIdAttr]) ||
		len(events[moduleAttr]) != len(events[typeAttr]) {
		return nil, fmt.Errorf("cannot filter interchain query messages because events attributes length does not match for events=%v", events)
	}

	messages := make([]queryEventMessage, 0, len(abciMessages))

	for idx, moduleName := range events[moduleAttr] {
		if moduleName != lidotypes.ModuleName {
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
	fmt.Printf("ProofMessage message_type=%s\n", m.messageType)
	switch m.messageType {
	case "x/staking/DelegatorDelegations":
		fmt.Printf("Unmarshal parameters=%s", m.parameters)
		var params getDelegatorDelegationsParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetDelegatorDelegations with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		proofs, height, err := r.proofer.GetDelegatorDelegations(ctx, uint64(0), r.targetChainPrefix, params.Delegator)
		if err != nil {
			return fmt.Errorf("could not get proof for GetDelegatorDelegations with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(height, m.queryId, proofs)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetDelegatorDelegations with query_id=%d: %w", m.queryId, err)
		}
	case "x/bank/GetBalance":
		var params getAllBalancesParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetBalance with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		proofs, height, err := r.proofer.GetBalance(ctx, r.targetChainPrefix, params.Addr, params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(height, m.queryId, proofs)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}
	case "x/bank/ExchangeRate":
		var params exchangeRateParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for ExchangeRate with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		supplyProofs, supplyHeight, err := r.proofer.GetSupply(ctx, params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for ExchangeRate with query_id=%d: %w", m.queryId, err)
		}

		delegationProofs, delegationHeight, err := r.proofer.GetDelegatorDelegations(ctx, supplyHeight, r.targetChainPrefix, params.Delegator)

		if delegationHeight != supplyHeight {
			return fmt.Errorf("heights for two parts of x/bank/ExchangeRate query does not match: delegationHeight=%d supplyHeight=%d", delegationHeight, supplyHeight)
		}

		err = r.submitter.SubmitProof(delegationHeight, m.queryId, append(supplyProofs, delegationProofs...))
		if err != nil {
			return fmt.Errorf("could not submit proof for ExchangeRate with query_id=%d: %w", m.queryId, err)
		}
	case "x/tx/RecipientTransactions":
		var params recipientTransactionsParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for RecipientTransactions with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		txProof, err := r.proofer.RecipientTransactions(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for RecipientTransactions with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitTxProof(m.queryId, txProof)
		if err != nil {
			return fmt.Errorf("could not submit proof for RecipientTransactions with query_id=%d: %w", m.queryId, err)
		}

	case "x/distribution/CalculateDelegationRewards":
		return fmt.Errorf("could not relay not implemented query x/distribution/CalculateDelegationRewards")

	default:
		return fmt.Errorf("unknown query message type=%s", m.messageType)
	}

	return nil
}
