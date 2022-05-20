package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	"github.com/lidofinance/cosmos-query-relayer/internal/submit"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"strconv"
)

type Relayer struct {
	proofer           proof.Proofer
	submitter         submit.Submitter
	targetChainPrefix string
	sender            string
}

func NewRelayer(proofer proof.Proofer, submitter submit.Submitter, targetChainPrefix string, sender string) Relayer {
	return Relayer{proofer: proofer, submitter: submitter, targetChainPrefix: targetChainPrefix, sender: sender}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) {
	messages := filterInterchainQueryMessagesFromEvent(event)

	for _, m := range messages {
		err := r.proofMessage(ctx, m)
		if err != nil {
			fmt.Printf("\ncould not process message query_id=%d err=%s\n", m.queryId, err)
		}
	}
}

// TODO: continue on errors or return if this happens?
func filterInterchainQueryMessagesFromEvent(event coretypes.ResultEvent) []queryEventMessage {
	abciMessages := make([]abci.Event, 0)
	for _, e := range event.Events {
		if e.Type == "message" {
			abciMessages = append(abciMessages, e)
		}
	}

	messages := make([]queryEventMessage, 0, len(abciMessages))
	for _, m := range abciMessages {
		queryIdStr, err := tryFindInEvent(m.GetAttributes(), "query_id")
		if err != nil {
			//fmt.Printf("couldn't find key in event: %s\n", err)
			continue
		}
		queryId, err := strconv.ParseUint(queryIdStr, 10, 64)
		if err != nil {
			// TODO: invalid query_id: %s?
			continue
		}

		messageType, err := tryFindInEvent(m.GetAttributes(), "type")
		if err != nil {
			//fmt.Printf("couldn't find key in event: %s\n", err)
			continue
		}

		parameters, err := tryFindInEvent(m.GetAttributes(), "parameters")
		if err != nil {
			//fmt.Printf("couldn't find key in event: %s\n", err)
			continue
		}

		messages = append(messages,
			queryEventMessage{queryId: queryId, messageType: messageType, parameters: parameters})
	}

	return messages
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

		proofs, height, err := r.proofer.GetDelegatorDelegations(ctx, r.targetChainPrefix, params.Delegator)
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
	case "x/tx/RecipientTransactions":
		var params recipientTransactionsParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for RecipientTransactions with params=%s query_id=%d: %w", m.parameters, m.queryId, err)
		}

		txProof, err := r.proofer.RecipientTransactions(ctx, params)
		if err != nil {
			return fmt.Errorf("could not get proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}

		err = r.submitter.SubmitTxProof(m.queryId, txProof)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetBalance with query_id=%d: %w", m.queryId, err)
		}
	case "x/bank/ExchangeRate":
	//	TODO
	case "x/distribution/CalculateDelegationRewards":
	//	TODO: not sure if needed

	default:
		return fmt.Errorf("unknown query message type=%s", m.messageType)
	}

	return nil
}

func tryFindInEvent(attributes []abci.EventAttribute, key string) (string, error) {
	for _, attr := range attributes {
		if attr.GetKey() == key {
			return attr.GetValue(), nil
		}
	}

	return "", fmt.Errorf("no attribute found with key=%s", key)
}
