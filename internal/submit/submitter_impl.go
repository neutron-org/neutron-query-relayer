package submit

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	neutrontypes "github.com/neutron-org/neutron/v4/x/interchainqueries/types"
)

// SubmitterImpl can submit proofs using `sender` as the transaction transport mechanism
type SubmitterImpl struct {
	sender           *TxSender
	allowKVCallbacks bool
	clientID         string
}

func NewSubmitterImpl(sender *TxSender, allowKVCallbacks bool, clientID string) *SubmitterImpl {
	return &SubmitterImpl{sender: sender, allowKVCallbacks: allowKVCallbacks, clientID: clientID}
}

// SubmitKVProof submits query with proof back to Neutron chain
func (si *SubmitterImpl) SubmitKVProof(
	ctx context.Context,
	height, revision, queryId uint64,
	proof []*neutrontypes.StorageValue,
	updateClientMsg sdk.Msg,
) error {
	msgs, err := si.buildProofMsg(height, revision, queryId, si.allowKVCallbacks, proof)
	if err != nil {
		return fmt.Errorf("could not build proof msg: %w", err)
	}

	msgs = append([]sdk.Msg{updateClientMsg}, msgs...)
	_, err = si.sender.Send(ctx, msgs)
	return err
}

// SubmitTxProof submits tx query with proof back to Neutron chain
func (si *SubmitterImpl) SubmitTxProof(ctx context.Context, queryId uint64, proof *neutrontypes.Block) (string, error) {
	msgs, err := si.buildTxProofMsg(queryId, proof)
	if err != nil {
		return "", fmt.Errorf("could not build tx proof msg: %w", err)
	}

	return si.sender.Send(ctx, msgs)
}

func (si *SubmitterImpl) buildProofMsg(height, revision, queryId uint64, allowKVCallbacks bool, proof []*neutrontypes.StorageValue) ([]sdk.Msg, error) {
	senderAddr, err := si.sender.SenderAddr()
	if err != nil {
		return nil, fmt.Errorf("could not fetch sender addr for building proof msg: %w", err)
	}

	queryResult := neutrontypes.QueryResult{
		Height:           height,
		KvResults:        proof,
		Revision:         revision,
		AllowKvCallbacks: allowKVCallbacks,
	}

	msg := neutrontypes.MsgSubmitQueryResult{QueryId: queryId, Sender: senderAddr, Result: &queryResult, ClientId: si.clientID}

	err = msg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid proof message for query=%d: %w", queryId, err)
	}

	return []sdk.Msg{&msg}, nil
}

func (si *SubmitterImpl) buildTxProofMsg(queryId uint64, proof *neutrontypes.Block) ([]sdk.Msg, error) {
	senderAddr, err := si.sender.SenderAddr()
	if err != nil {
		return nil, fmt.Errorf("could not fetch sender addr for building tx proof msg: %w", err)
	}

	queryResult := neutrontypes.QueryResult{
		Height:    0, // NOTE: cannot use nil because it's not pointer :(
		KvResults: nil,
		Block:     proof,
	}
	msg := neutrontypes.MsgSubmitQueryResult{QueryId: queryId, Sender: senderAddr, Result: &queryResult, ClientId: si.clientID}

	err = msg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid tx proof message: %w", err)
	}

	return []sdk.Msg{&msg}, nil
}
