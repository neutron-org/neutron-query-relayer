package submitter

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
	lidotypes "github.com/lidofinance/interchain-adapter/x/interchainqueries/types"
	"github.com/tendermint/tendermint/proto/tendermint/crypto"
)

type ProofSubmitter struct {
	txSubmitter *chain.TxSubmitter
}

func NewProofSubmitter(txSubmitter *chain.TxSubmitter) *ProofSubmitter {
	return &ProofSubmitter{
		txSubmitter: txSubmitter,
	}
}

// SubmitProof submits query with proof back to lido chain
func (cc *ProofSubmitter) SubmitProof(sender string, height uint64, queryId uint64, proof []proofer.StorageValue) error {
	msgs, err := cc.buildProofMsg(sender, height, queryId, proof)
	if err != nil {
		return err
	}
	return cc.txSubmitter.BuildAndSendTx(sender, msgs)
}

func (cc *ProofSubmitter) SubmitTxProof(sender string, queryId uint64, proof []proofer.CompleteTransactionProof) error {
	msgs, err := cc.buildTxProofMsg(sender, queryId, proof)
	if err != nil {
		return err
	}
	return cc.txSubmitter.BuildAndSendTx(sender, msgs)
}

// SendCoins test func
func (cc *ProofSubmitter) SendCoins(address1, address2 string) error {
	fmt.Printf("About to Send coins from / to =: %v / %v\n", address1, address2)

	msgs, err := cc.buildSendMsgs(address1, address2)
	if err != nil {
		return err
	}

	return cc.txSubmitter.BuildAndSendTx(address1, msgs)
}

func (cc *ProofSubmitter) buildProofMsg(sender string, height uint64, queryId uint64, proof []proofer.StorageValue) ([]types.Msg, error) {
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

	msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Height: height, Sender: sender, KVResults: res}

	err := msg.ValidateBasic()
	if err != nil {
		return nil, fmt.Errorf("invalid proof message: %w", err)
	}

	return []types.Msg{&msg}, nil
}

func (cc *ProofSubmitter) buildTxProofMsg(sender string, queryId uint64, proof []proofer.CompleteTransactionProof) ([]types.Msg, error) {
	//TODO
	//res := make([]*lidotypes.StorageValue, 0, len(proof))
	//for _, item := range proof {
	//	res = append(res, &lidotypes.StorageValue{
	//		StoragePrefix: item.StoragePrefix,
	//		Key:           item.Key,
	//		Value:         item.Value,
	//		Proof: &crypto.ProofOps{
	//			Ops: item.Proofs,
	//		},
	//	})
	//}
	//
	//msg := lidotypes.MsgSubmitQueryResult{QueryId: queryId, Height: height, Sender: sender, KVResults: res}
	//
	//err := msg.ValidateBasic()
	//if err != nil {
	//	return nil, err
	//}
	//
	//return []types.Msg{&msg}, nil
	return nil, nil
}

func (cc *ProofSubmitter) buildSendMsgs(address1, address2 string) ([]types.Msg, error) {
	amount := types.NewCoins(types.NewInt64Coin("uluna", 100000))
	msg := &banktypes.MsgSend{FromAddress: address1, ToAddress: address2, Amount: amount}

	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}
	//signedMsg := msg.GetSignBytes()
	//fmt.Printf("\nBuilt Tx info: %+v\n\n", string(signedMsg))

	return []types.Msg{msg}, nil
}
