package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	"github.com/lidofinance/cosmos-query-relayer/internal/submitter"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"strconv"
)

type Relayer struct {
	querier           *proofer.ProofQuerier
	submitter         *submitter.ProofSubmitter
	targetChainPrefix string
	sender            string
}

type QueryEventMessage struct {
	queryId     uint64
	messageType string
	parameters  string
}

type GetDelegatorDelegationsParameters struct {
	Delegator string `json:"delegator"`
}

type GetAllBalancesParams struct {
	Addr  string `json:"addr"`
	Denom string `json:"denom"`
}

type RecipientTransactionsParams struct {
	Recipient string `json:"recipient"`
}

func NewRelayer(querier *proofer.ProofQuerier, submitter *submitter.ProofSubmitter, targetChainPrefix string, sender string) Relayer {
	return Relayer{querier: querier, submitter: submitter, targetChainPrefix: targetChainPrefix, sender: sender}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) {
	messages := filterInterchainQueryMessagesFromEvent(event)
	fmt.Println("Got messages:")
	for _, m := range messages {
		fmt.Printf("%+v\n", m)
	}

	for _, m := range messages {
		err := r.ProofMessage(ctx, m)
		if err != nil {
			fmt.Printf("\ncould not process message query_id=%s err=%s\n", m.queryId, err)
		}
	}
}

// TODO: continue on errors or return if this happens?
func filterInterchainQueryMessagesFromEvent(event coretypes.ResultEvent) []QueryEventMessage {
	abciMessages := make([]abci.Event, 0)
	for _, e := range event.Events {
		if e.Type == "message" {
			abciMessages = append(abciMessages, e)
		}
	}

	messages := make([]QueryEventMessage, 0, len(abciMessages))
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
			QueryEventMessage{queryId: queryId, messageType: messageType, parameters: parameters})
	}

	return messages
}

func (r Relayer) ProofMessage(ctx context.Context, m QueryEventMessage) error {
	fmt.Printf("ProofMessage message_type=%s", m.messageType)
	switch m.messageType {
	case "x/staking/DelegatorDelegations":
		fmt.Printf("Unmarshal parameters=%s", m.parameters)
		var params GetDelegatorDelegationsParameters
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetDelegatorDelegations with params=%s query_id=%s: %w", m.parameters, m.queryId, err)
		}

		proof, height, err := proofs.GetDelegatorDelegations(ctx, r.querier, r.targetChainPrefix, params.Delegator)
		if err != nil {
			return fmt.Errorf("could not get proof for GetDelegatorDelegations with query_id=%s: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(r.sender, height, m.queryId, proof)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetDelegatorDelegations with query_id=%s: %w", m.queryId, err)
		}
	case "x/bank/GetBalance":
		var params GetAllBalancesParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for GetBalance with params=%s query_id=%s: %w", m.parameters, m.queryId, err)
		}

		proof, height, err := proofs.GetBalance(ctx, r.querier, r.targetChainPrefix, params.Addr, params.Denom)
		if err != nil {
			return fmt.Errorf("could not get proof for GetBalance with query_id=%s: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(r.sender, height, m.queryId, proof)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetBalance with query_id=%s: %w", m.queryId, err)
		}
	case "x/tx/RecipientTransactions":
		var params RecipientTransactionsParams
		err := json.Unmarshal([]byte(m.parameters), &params)
		if err != nil {
			return fmt.Errorf("could not unmarshal parameters for RecipientTransactions with params=%s query_id=%s: %w", m.parameters, m.queryId, err)
		}

		txProof, height, err := proofs.RecipientTransactions(ctx, r.querier, params.Recipient)
		if err != nil {
			return fmt.Errorf("could not get proof for GetBalance with query_id=%s: %w", m.queryId, err)
		}

		err = r.submitter.SubmitTxProof(r.sender, height, m.queryId, txProof)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetBalance with query_id=%s: %w", m.queryId, err)
		}
	case "x/bank/ExchangeRate":
	//	TODO

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