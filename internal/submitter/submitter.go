package submitter

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

type TxSubmitter struct {
	ctx       context.Context
	txf       tx.Factory
	codec     Codec
	rpcClient rpcclient.Client
	chainID   string
}

func NewTxSubmitter(ctx context.Context, rpcClient rpcclient.Client, chainID string, codec Codec, gasAdj float64, gasPrices string) (*TxSubmitter, error) {
	// TODO: pick key backend: https://docs.cosmos.network/master/run-node/keyring.html
	keybase, err := keyring.New(chainID, "test", "/Users/nhpd/lido/cosmos-query-relayer/keys", nil, codec.Marshaller)
	if err != nil {
		return nil, err
	}
	return &TxSubmitter{
		ctx:       ctx,
		codec:     codec,
		txf:       TxFactory(chainID, codec, gasAdj, gasPrices, keybase),
		rpcClient: rpcClient,
		chainID:   chainID,
	}, nil
}

func TxFactory(chainId string, codec Codec, gasAdjustment float64, gasPrices string, keybase keyring.Keyring) tx.Factory {
	return tx.Factory{}.
		//WithAccountRetriever(cc).
		WithChainID(chainId).
		WithTxConfig(codec.TxConfig).
		WithGasAdjustment(gasAdjustment).
		WithGasPrices(gasPrices).
		WithKeybase(keybase).
		WithSignMode(mode)
}

// TODO: submits query with proof back to lido chain
func (cc *TxSubmitter) Send(prefix, address1, address2 string) error {
	fmt.Printf("Trying to send from / to =: %v / %v\n", address1, address2)
	bz, err := cc.buildTxBz(prefix, address1, address2)
	if err != nil {
		return err
	}
	_, err = cc.calculateGas(bz)
	if err != nil {
		return err
	}
	res, err := cc.rpcClient.BroadcastTxSync(cc.ctx, bz)

	fmt.Printf("Broadcast result: res=%+v err=%+v", res, err)

	return nil
}

//var mode = signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON

var mode = signing.SignMode_SIGN_MODE_DIRECT

func (cc *TxSubmitter) buildTxBz(prefix, address1, address2 string) ([]byte, error) {
	//bz1, err := types.GetFromBech32(address1, prefix)
	//if err != nil {
	//	return nil, err
	//}
	//bz2, err := types.GetFromBech32(address2, prefix)
	//if err != nil {
	//	return nil, err
	//}
	// TODO: build needed msgs
	amount := types.NewCoins(types.NewInt64Coin("uatom", 1))
	//msg := banktypes.NewMsgSend(bz1, bz2, amount)
	msg := &banktypes.MsgSend{FromAddress: address1, ToAddress: address2, Amount: amount}
	err := msg.ValidateBasic()
	if err != nil {
		//fmt.Printf("\n\n\nvalidate error\n\n\n")
		return nil, err
	}
	signedTx := msg.GetSignBytes()
	fmt.Printf("\nSIGNED Tx: %+v\n\n", string(signedTx))
	txBuilder := cc.codec.TxConfig.NewTxBuilder()

	//aminoConfig := legacytx.StdTxConfig{Cdc: cc.codec.Amino}
	//txBuilder := aminoConfig.NewTxBuilder()
	err = txBuilder.SetMsgs(msg)
	//signatures :=
	//txBuilder.SetSignatures(signatures)
	if err != nil {
		fmt.Printf("set msgs failure")
		return nil, err
	}

	//txBuilder.SetGasLimit(30000)
	//txBuilder.SetFeeAmount(...)
	txBuilder.SetMemo("bob to alice")
	//txBuilder.SetTimeoutHeight(...)

	err = tx.Sign(cc.txf, "bob", txBuilder, true)

	if err != nil {
		return nil, err
	}

	bz, err := cc.codec.TxConfig.TxEncoder()(txBuilder.GetTx())
	return bz, err
}

func (cc *TxSubmitter) calculateGas(txBytes []byte) (uint64, error) {
	//BuildSimTx(txf, msgs...)
	// We then call the Simulate method on this client.
	simQuery := abci.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: txBytes,
	}
	res, err := cc.rpcClient.ABCIQuery(cc.ctx, simQuery.Path, simQuery.Data)
	if err != nil {
		return 0, err
	}

	var simRes txtypes.SimulateResponse
	if err := simRes.Unmarshal(res.Response.Value); err != nil {
		return 0, err
	}

	fmt.Printf("Simulate Result: %+v\n", simRes) // Prints estimated gas used.

	return uint64(cc.txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
}

//func (cc *ChainClient) CalculateGas(ctx context.Context, txf tx.Factory, msgs ...sdk.Msg) (txtypes.SimulateResponse, uint64, error) {
//	var txBytes []byte
//	if err := retry.Do(func() error {
//		var err error
//		txBytes, err = BuildSimTx(txf, msgs...)
//		if err != nil {
//			return err
//		}
//		return nil
//	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
//		return txtypes.SimulateResponse{}, 0, err
//	}
//
//	simQuery := abci.RequestQuery{
//		Path: "/cosmos.tx.v1beta1.Service/Simulate",
//		Data: txBytes,
//	}
//
//	var res abci.ResponseQuery
//	if err := retry.Do(func() error {
//		var err error
//		res, err = cc.QueryABCI(ctx, simQuery)
//		if err != nil {
//			return err
//		}
//		return nil
//	}, retry.Context(ctx), RtyAtt, RtyDel, RtyErr); err != nil {
//		return txtypes.SimulateResponse{}, 0, err
//	}
//
//	var simRes txtypes.SimulateResponse
//	if err := simRes.Unmarshal(res.Value); err != nil {
//		return txtypes.SimulateResponse{}, 0, err
//	}
//
//	return simRes, uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
//}
