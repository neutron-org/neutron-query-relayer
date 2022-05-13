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
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

//var mode = signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON

var mode = signing.SignMode_SIGN_MODE_DIRECT

type TxSubmitter struct {
	ctx           context.Context
	baseTxf       tx.Factory
	codec         Codec
	rpcClient     rpcclient.Client
	chainID       string
	addressPrefix string
}

func NewTxSubmitter(ctx context.Context, rpcClient rpcclient.Client, chainID string, codec Codec, gasAdj float64, gasPrices string, addressPrefix string, keyringRootDir string) (*TxSubmitter, error) {
	// TODO: pick key backend: https://docs.cosmos.network/master/run-node/keyring.html
	keybase, err := keyring.New(chainID, "test", keyringRootDir, nil, codec.Marshaller)
	if err != nil {
		return nil, err
	}
	baseTxf := tx.Factory{}.
		WithChainID(chainID).
		WithTxConfig(codec.TxConfig).
		WithGasAdjustment(gasAdj).
		WithGasPrices(gasPrices).
		WithKeybase(keybase).
		WithSignMode(mode)
	return &TxSubmitter{
		ctx:           ctx,
		codec:         codec,
		baseTxf:       baseTxf,
		rpcClient:     rpcClient,
		chainID:       chainID,
		addressPrefix: addressPrefix,
	}, nil
}

// TODO: submits query with proof back to lido chain
func (cc *TxSubmitter) Send(address1, address2 string) error {
	fmt.Printf("About to Send coins from / to =: %v / %v\n", address1, address2)

	msgs, err := cc.buildMsgs(address1, address2)
	if err != nil {
		return err
	}

	account, err := cc.QueryAccount(address1)
	if err != nil {
		return err
	}

	txf := cc.baseTxf.
		WithAccountNumber(account.AccountNumber).
		WithSequence(account.Sequence)

	gasNeeded, err := cc.calculateGas(txf, msgs...)
	if err != nil {
		return err
	}

	txf = txf.WithGas(gasNeeded)

	bz, err := cc.buildTxBz(txf, msgs, address1, gasNeeded)
	if err != nil {
		return err
	}
	res, err := cc.rpcClient.BroadcastTxSync(cc.ctx, bz)

	fmt.Printf("Broadcast result: code=%+v log=%v err=%+v", res.Code, res.Log, err)

	return nil
}

func (cc *TxSubmitter) buildMsgs(address1, address2 string) ([]types.Msg, error) {
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

func (cc *TxSubmitter) buildTxBz(txf tx.Factory, msgs []types.Msg, feePayerAddress string, gasAmount uint64) ([]byte, error) {
	txBuilder := cc.codec.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	if err != nil {
		fmt.Printf("set msgs failure")
		return nil, err
	}

	txBuilder.SetGasLimit(gasAmount) // TODO: correct?
	txBuilder.SetMemo("bob to alice")

	feePayerBz, err := types.GetFromBech32(feePayerAddress, cc.addressPrefix)
	if err != nil {
		return nil, err
	}
	txBuilder.SetFeePayer(feePayerBz)
	// TODO: correct?
	txBuilder.SetFeeAmount(types.NewCoins(types.NewInt64Coin("uluna", 2000)))
	//txBuilder.SetFeeGranter()
	//txBuilder.SetTimeoutHeight(...)

	fmt.Printf("\nAbout to sign with txf: %+v\n\n", txf)
	err = tx.Sign(txf, "bob", txBuilder, true)

	if err != nil {
		return nil, err
	}

	bz, err := cc.codec.TxConfig.TxEncoder()(txBuilder.GetTx())
	return bz, err
}

func (cc *TxSubmitter) calculateGas(txf tx.Factory, msgs ...types.Msg) (uint64, error) {
	simulation, err := cc.BuildSimTx(txf, msgs...)
	if err != nil {
		return 0, err
	}
	// We then call the Simulate method on this client.
	simQuery := abci.RequestQuery{
		Path: "/cosmos.tx.v1beta1.Service/Simulate",
		Data: simulation,
	}
	res, err := cc.rpcClient.ABCIQueryWithOptions(cc.ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return 0, err
	}

	var simRes txtypes.SimulateResponse

	if err := simRes.Unmarshal(res.Response.Value); err != nil {
		return 0, err
	}
	if simRes.GasInfo == nil {
		return 0, fmt.Errorf("no result in simulation response: %+v", simRes)
	}

	return uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
}

// BuildSimTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func (cc *TxSubmitter) BuildSimTx(txf tx.Factory, msgs ...types.Msg) ([]byte, error) {
	txb, err := cc.baseTxf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: &secp256k1.PubKey{},
		Data: &signing.SingleSignatureData{
			SignMode: cc.baseTxf.SignMode(),
		},
		Sequence: txf.Sequence(),
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, err
	}

	bz, err := cc.codec.TxConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return nil, nil
	}
	simReq := txtypes.SimulateRequest{TxBytes: bz}
	return simReq.Marshal()
}

// QueryAccount returns BaseAccount for given account address
func (cc *TxSubmitter) QueryAccount(address string) (*authtypes.BaseAccount, error) {
	request := authtypes.QueryAccountRequest{Address: address}
	req, err := request.Marshal()
	if err != nil {
		return nil, err
	}
	// We then call the Simulate method on this client.
	simQuery := abci.RequestQuery{
		Path: "/cosmos.auth.v1beta1.Query/Account",
		Data: req,
	}
	res, err := cc.rpcClient.ABCIQueryWithOptions(cc.ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return nil, err
	}

	var response authtypes.QueryAccountResponse
	if err := response.Unmarshal(res.Response.Value); err != nil {
		return nil, err
	}

	var account authtypes.BaseAccount
	err = account.Unmarshal(response.Account.Value)

	if err != nil {
		return nil, err
	}

	return &account, nil
}
