package relay

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer/proofs"
	"github.com/lidofinance/cosmos-query-relayer/internal/submitter"
	itypes "github.com/lidofinance/interchain-adapter/x/interchainqueries/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

type Relayer struct {
	querier           *proofer.ProofQuerier
	submitter         *submitter.ProofSubmitter
	targetChainPrefix string
	submitTxAuthor    string
}

type QueryEventMessage struct {
	queryId     string
	messageType string
	parameters  string
}

func NewRelayer(querier *proofer.ProofQuerier, submitter *submitter.ProofSubmitter, targetChainPrefix string, submitTxAuthor string) Relayer {
	return Relayer{querier: querier, submitter: submitter, targetChainPrefix: targetChainPrefix, submitTxAuthor: submitTxAuthor}
}

func (r Relayer) Proof(ctx context.Context, event coretypes.ResultEvent) {
	var _ itypes.MsgRegisterInterchainQuery
	// TODO:
	// - how event will look like
	// - how submit response will look like

	messages := filterInterchainQueryMessagesFromEvent(event)
	fmt.Println("Got messages:")
	for _, m := range messages {
		fmt.Printf("%+v\n", m)
	}

	for _, m := range messages {
		err := r.ProofMessage(ctx, m)
		if err != nil {
			//	TODO: log
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
		queryId, err := tryFindInEvent(m.GetAttributes(), "query_id")
		if err != nil {
			//fmt.Printf("couldn't find key in event: %s\n", err)
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

// TODO: make some logging and monitoring there
func (r Relayer) ProofMessage(ctx context.Context, m QueryEventMessage) error {
	switch m.messageType {
	case "cosmos.staking.Query/DelegatorDelegations":
		delegatorAddress := m.parameters
		proof, err := proofs.GetDelegatorDelegations(ctx, r.querier, r.targetChainPrefix, delegatorAddress)
		if err != nil {
			return fmt.Errorf("could not get proof for GetDelegatorDelegations with query_id=%s: %w", m.queryId, err)
		}

		err = r.submitter.SubmitProof(r.submitTxAuthor, proof)
		if err != nil {
			return fmt.Errorf("could not submit proof for GetDelegatorDelegations with query_id=%s: %w", m.queryId, err)
		}

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
