package submit

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	lidotypes "github.com/lidofinance/interchain-adapter/x/interchainqueries/types"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

type ProofSubmitter struct {
	sender      string
	txSubmitter *chain.TxSubmitter
}

func NewProofSubmitter(sender string, txSubmitter *chain.TxSubmitter) *ProofSubmitter {
	return &ProofSubmitter{
		sender, txSubmitter,
	}
}

// SubmitProof submits query with proof back to lido chain
func (cc *ProofSubmitter) SubmitProof(height uint64, queryId uint64, proof []proof.StorageValue) error {
	msgs, err := cc.buildProofMsg(height, queryId, proof)
	if err != nil {
		return err
	}
	return cc.txSubmitter.BuildAndSendTx(cc.sender, msgs)
}

func (cc *ProofSubmitter) SubmitTxProof(queryId uint64, proof []proof.CompleteTransactionProof) error {
	msgs, err := cc.buildTxProofMsg(queryId, proof)
	if err != nil {
		return err
	}
	return cc.txSubmitter.BuildAndSendTx(cc.sender, msgs)
}

func (cc *ProofSubmitter) buildProofMsg(height uint64, queryId uint64, proof []proof.StorageValue) ([]types.Msg, error) {
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

	msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Height: height, Sender: cc.sender, KVResults: res}

	err := msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("invalid proof message: %w", err)
	}

	return []types.Msg{&msg}, nil
}

func (cc *ProofSubmitter) buildTxProofMsg(queryId uint64, proof []proof.CompleteTransactionProof) ([]types.Msg, error) {
	// TODO
	return nil, nil
}
