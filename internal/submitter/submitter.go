package submitter

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/chain"
	"github.com/lidofinance/cosmos-query-relayer/internal/proofer"
)

type ProofSubmitter struct {
	txSubmitter *chain.TxSubmitter
}

type InterchainQueryResultMsg struct {
	QueryData []byte
	QueryType string
	ZoneID    string
	Height    uint64 // height of queried chain when result was received

	//KVResults []StorageValue
}

func NewProofSubmitter(txSubmitter *chain.TxSubmitter) *ProofSubmitter {
	return &ProofSubmitter{
		txSubmitter: txSubmitter,
	}
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

// SubmitProof submits query with proof back to lido chain
func (cc *ProofSubmitter) SubmitProof(txAuthor string, proof []proofer.StorageValue) error {
	msgs, err := cc.buildProofMsg(proof)
	if err != nil {
		return err
	}
	return cc.txSubmitter.BuildAndSendTx(txAuthor, msgs)
}

func (cc *ProofSubmitter) buildProofMsg(proof []proofer.StorageValue) ([]types.Msg, error) {
	// TODO
	return nil, nil
}

func (cc *ProofSubmitter) buildSendMsgs(address1, address2 string) ([]types.Msg, error) {
	amount := types.NewCoins(types.NewInt64Coin("uluna", 100000))
	msg := &banktypes.MsgSend{FromAddress: address1, ToAddress: address2, Amount: amount}

	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}
	signedMsg := msg.GetSignBytes()
	fmt.Printf("\nBuilt Tx info: %+v\n\n", string(signedMsg))

	return []types.Msg{msg}, nil
}
