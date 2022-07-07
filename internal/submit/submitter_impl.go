package submit

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	lidotypes "github.com/neutron-org/neutron/x/interchainqueries/types"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

// SubmitterImpl can submit proofs using `sender` as the transaction transport mechanism
type SubmitterImpl struct {
	sender *TxSender
}

func NewSubmitterImpl(sender *TxSender) *SubmitterImpl {
	return &SubmitterImpl{sender: sender}
}

// SubmitProof submits query with proof back to lido chain
func (si *SubmitterImpl) SubmitProof(ctx context.Context, height uint64, revision uint64, queryId uint64, proof []proof.StorageValue, updateClientMsg sdk.Msg) error {
	msgs, err := si.buildProofMsg(height, revision, queryId, proof)
	if err != nil {
		return fmt.Errorf("could not build proof msg: %w", err)
	}

	msgs = append([]sdk.Msg{updateClientMsg}, msgs...)

	return si.sender.Send(ctx, msgs)
}

// SubmitTxProof submits tx query with proof back to lido chain
func (si *SubmitterImpl) SubmitTxProof(ctx context.Context, queryId uint64, clientID string, proof []*lidotypes.Block) error {
	msgs, err := si.buildTxProofMsg(queryId, clientID, proof)
	if err != nil {
		return fmt.Errorf("could not build tx proof msg: %w", err)
	}

	return si.sender.Send(ctx, msgs)
}

func (si *SubmitterImpl) buildProofMsg(height uint64, revision uint64, queryId uint64, proof []proof.StorageValue) ([]sdk.Msg, error) {
	res := make([]*lidotypes.StorageValue, 0, len(proof))
	for _, item := range proof {
		res = append(res, &lidotypes.StorageValue{
			StoragePrefix: item.StoragePrefix,
			Key:           item.Key,
			Value:         item.Value,
			Proof: &crypto.ProofOps{
				Ops: item.Proofs,
			},
		})
	}

	senderAddr, err := si.sender.SenderAddr()
	if err != nil {
		return nil, fmt.Errorf("could not fetch sender addr for building proof msg: %w", err)
	}

	queryResult := lidotypes.QueryResult{
		Height:    height,
		KvResults: res,
		Revision:  revision,
	}
	msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Sender: senderAddr, Result: &queryResult}

	err = msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("invalid proof message for query=%d: %w", queryId, err)
	}

	return []sdk.Msg{&msg}, nil
}

func (si *SubmitterImpl) buildTxProofMsg(queryId uint64, clientID string, proof []*lidotypes.Block) ([]sdk.Msg, error) {
	senderAddr, err := si.sender.SenderAddr()
	if err != nil {
		return nil, fmt.Errorf("could not fetch sender addr for building tx proof msg: %w", err)
	}

	queryResult := lidotypes.QueryResult{
		Height:    0, // NOTE: cannot use nil because it's not pointer :(
		KvResults: nil,
		Blocks:    proof,
	}
	msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Sender: senderAddr, Result: &queryResult, ClientId: clientID}

	err = msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("invalid tx proof message: %w", err)
	}

	return []sdk.Msg{&msg}, nil
}
