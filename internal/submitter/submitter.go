package submitter

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

//var mode = signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON
var height int64 = 0

var mode = signing.SignMode_SIGN_MODE_DIRECT

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
func (cc *TxSubmitter) Send(address1, address2 string) error {
	fmt.Printf("Trying to send from / to =: %v / %v\n", address1, address2)
	msgs, err := cc.buildMsgs(address1, address2)
	if err != nil {
		return err
	}
	gasNeeded, err := cc.calculateGas(msgs...)
	if err != nil {
		return err
	}
	_, err = cc.buildTxBz(msgs, gasNeeded)
	if err != nil {
		return err
	}
	//res, err := cc.rpcClient.BroadcastTxSync(cc.ctx, bz)

	//fmt.Printf("Broadcast result: res=%+v err=%+v", res, err)

	return nil
}

func (cc *TxSubmitter) buildMsgs(address1, address2 string) ([]types.Msg, error) {
	amount := types.NewCoins(types.NewInt64Coin("uatom", 1))
	msg := &banktypes.MsgSend{FromAddress: address1, ToAddress: address2, Amount: amount}

	err := msg.ValidateBasic()
	if err != nil {
		return nil, err
	}
	signedMsg := msg.GetSignBytes()
	fmt.Printf("\nSIGNED Tx: %+v\n\n", string(signedMsg))

	return []types.Msg{msg}, nil
}

func (cc *TxSubmitter) buildTxBz(msgs []types.Msg, gasAmount uint64) ([]byte, error) {
	txBuilder := cc.codec.TxConfig.NewTxBuilder()
	//aminoConfig := legacytx.StdTxConfig{Cdc: cc.codec.Amino}
	//txBuilder := aminoConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	//signatures :=
	//txBuilder.SetSignatures(signatures)
	if err != nil {
		fmt.Printf("set msgs failure")
		return nil, err
	}

	cc.txf.WithGas(gasAmount)
	txBuilder.SetGasLimit(gasAmount) // TODO: correct?
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

func (cc *TxSubmitter) calculateGas(msgs ...types.Msg) (uint64, error) {
	simulation, err := cc.BuildSimTx(msgs...)
	if err != nil {
		return 0, err
	}
	// We then call the Simulate method on this client.
	simQuery := abci.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: simulation,
	}
	opts := rpcclient.ABCIQueryOptions{
		Height: height,
		Prove:  false,
	}
	res, err := cc.rpcClient.ABCIQueryWithOptions(cc.ctx, simQuery.Path, simQuery.Data, opts)
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

// protoTxProvider is a type which can provide a proto transaction. It is a
// workaround to get access to the wrapper TxBuilder's method GetProtoTx().
type protoTxProvider interface {
	GetProtoTx() *txtypes.Tx
}

// BuildSimTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func (cc *TxSubmitter) BuildSimTx(msgs ...types.Msg) ([]byte, error) {
	txb, err := cc.txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: &secp256k1.PubKey{},
		Data: &signing.SingleSignatureData{
			SignMode: cc.txf.SignMode(),
		},
		Sequence: cc.txf.Sequence(),
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, err
	}

	protoProvider, ok := txb.(protoTxProvider)
	if !ok {
		return nil, fmt.Errorf("cannot simulate amino tx")
	}

	simReq := txtypes.SimulateRequest{Tx: protoProvider.GetProtoTx()}
	return simReq.Marshal()
}
