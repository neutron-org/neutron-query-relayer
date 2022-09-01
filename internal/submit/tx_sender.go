package submit

import (
	"context"
	"fmt"
	"sync"

	"github.com/cosmos/cosmos-sdk/api/tendermint/abci"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtxtypes "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"

	"github.com/neutron-org/neutron-query-relayer/internal/config"
)

const (
	accountQueryPath  = "/cosmos.auth.v1beta1.Query/Account"
	simulateQueryPath = "/cosmos.tx.v1beta1.Service/Simulate"
)

type TxSender struct {
	lock          sync.Mutex
	sequence      uint64
	accountNumber uint64
	keybase       keyring.Keyring
	baseTxf       tx.Factory
	txConfig      client.TxConfig
	rpcClient     rpcclient.Client
	chainID       string
	addressPrefix string
	signKeyName   string
	gasPrices     string
	gasLimit      uint64
}

func TestKeybase(chainID string, keyringRootDir string) (keyring.Keyring, error) {
	keybase, err := keyring.New(chainID, "test", keyringRootDir, nil)
	if err != nil {
		return keybase, fmt.Errorf("error creating keybase for chainId=%s and keyringRootDir=%s: %w", chainID, keyringRootDir, err)
	}

	return keybase, nil
}

func NewTxSender(rpcClient rpcclient.Client, marshaller codec.ProtoCodecMarshaler, keybase keyring.Keyring, cfg config.NeutronChainConfig) (*TxSender, error) {
	txConfig := authtxtypes.NewTxConfig(marshaller, authtxtypes.DefaultSignModes)
	baseTxf := tx.Factory{}.
		WithKeybase(keybase).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT).
		WithTxConfig(txConfig).
		WithChainID(cfg.ChainID).
		WithGasAdjustment(cfg.GasAdjustment).
		WithGasPrices(cfg.GasPrices)

	txs := &TxSender{
		lock:          sync.Mutex{},
		keybase:       keybase,
		txConfig:      txConfig,
		baseTxf:       baseTxf,
		rpcClient:     rpcClient,
		chainID:       cfg.ChainID,
		addressPrefix: cfg.ChainPrefix,
		signKeyName:   cfg.SignKeyName,
		gasPrices:     cfg.GasPrices,
		gasLimit:      cfg.GasLimit,
	}
	err := txs.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to init tx sender: %w", err)
	}

	return txs, nil
}

func (txs *TxSender) Init() error {
	//TODO pass ctx as method arg
	ctx := context.Background()
	senderAddr, err := txs.SenderAddr()
	if err != nil {
		return fmt.Errorf("could not fetch sender addr: %w", err)
	}

	account, err := txs.queryAccount(ctx, senderAddr)
	if err != nil {
		return fmt.Errorf("error fetching account: %w", err)
	}
	txs.accountNumber = account.AccountNumber
	txs.sequence = account.Sequence
	return nil
}

// Send builds transaction with calculated input msgs, calculated gas and fees, signs it and submits to chain
func (txs *TxSender) Send(ctx context.Context, msgs []sdk.Msg) error {
	txs.lock.Lock()
	defer txs.lock.Unlock()

	txf := txs.baseTxf.
		WithAccountNumber(txs.accountNumber).
		WithSequence(txs.sequence)

	gasNeeded, err := txs.calculateGas(ctx, txf, msgs...)
	// TODO: reinit sender on error
	// {"level":"error","ts":1661995842.0200863,"caller":"relay/relayer.go:118","msg":"could not process message","quer$
	//_id":3,"error":"failed to process txs: failed to submit block: could not submit proof for tx with query_id=3: er$or calculating gas: no result in simulation response with log=\ngithub.com/cosmos/cosmos-sdk/baseapp.gRPCErrorTo$
	//DKError\n\tgithub.com/cosmos/cosmos-sdk@v0.45.4/baseapp/abci.go:590\ngithub.com/cosmos/cosmos-sdk/baseapp.(*Base$pp).handleQueryGRPC\n\tgithub.com/cosmos/cosmos-sdk@v0.45.4/baseapp/abci.go:579\ngithub.com/cosmos/cosmos-sdk/ba$
	//eapp.(*BaseApp).Query\n\tgithub.com/cosmos/cosmos-sdk@v0.45.4/baseapp/abci.go:421\ngithub.com/tendermint/tenderm$nt/abci/client.(*localClient).QuerySync\n\tgithub.com/tendermint/tendermint@v0.34.19/abci/client/local_client.go$
	//256\ngithub.com/tendermint/tendermint/proxy.(*appConnQuery).QuerySync\n\tgithub.com/tendermint/tendermint@v0.34.$9/proxy/app_conn.go:159\ngithub.com/tendermint/tendermint/rpc/core.ABCIQuery\n\tgithub.com/tendermint/tendermint$
	//v0.34.19/rpc/core/abci.go:20\nreflect.Value.call\n\treflect/value.go:556\nreflect.Value.Call\n\treflect/value.go$339\ngithub.com/tendermint/tendermint/rpc/jsonrpc/server.makeJSONRPCHandler.func1\n\tgithub.com/tendermint/tende$
	//mint@v0.34.19/rpc/jsonrpc/server/http_json_handler.go:96\ngithub.com/tendermint/tendermint/rpc/jsonrpc/server.ha$
	//dleInvalidJSONRPCPaths.func1\n\tgithub.com/tendermint/tendermint@v0.34.19/rpc/jsonrpc/server/http_json_handler.g$
	//:122\nnet/http.HandlerFunc.ServeHTTP\n\tnet/http/server.go:2084\nnet/http.(*ServeMux).ServeHTTP\n\tnet/http/serv$r.go:2462\ngithub.com/tendermint/tendermint/rpc/jsonrpc/server.maxBytesHandler.ServeHTTP\n\tgithub.com/tendermin$
	///tendermint@v0.34.19/rpc/jsonrpc/server/http_server.go:236\ngithub.com/tendermint/tendermint/rpc/jsonrpc/server.$ecoverAndLogHandler.func1\n\tgithub.com/tendermint/tendermint@v0.34.19/rpc/jsonrpc/server/http_server.go:209\nne$
	///http.HandlerFunc.ServeHTTP\n\tnet/http/server.go:2084\nnet/http.serverHandler.ServeHTTP\n\tnet/http/server.go:2$16\nnet/http.(*conn).serve\n\tnet/http/server.go:1966\naccount sequence mismatch, expected 22, got 19: incorrect
	//account sequence: invalid request code=18","stacktrace":"github.com/neutron-org/cosmos-query-relayer/internal/re$
	//ay.(*Relayer).Run\n\t/home/swelf/src/lido/cosmos-query-relayer/internal/relay/relayer.go:118\nmain.main.func2\n\$/home/swelf/src/lido/cosmos-query-relayer/cmd/cosmos_query_relayer/main.go:46"}
	// incorrect account sequence
	if err != nil {
		return fmt.Errorf("error calculating gas: %w", err)
	}

	if txs.gasLimit > 0 && gasNeeded > txs.gasLimit {
		return fmt.Errorf("exceeds gas limit: gas needed %d, gas limit %d", gasNeeded, txs.gasLimit)
	}

	txf = txf.
		WithGas(gasNeeded).
		WithGasPrices(txs.gasPrices)

	bz, err := txs.signAndBuildTxBz(txf, msgs)
	if err != nil {
		return fmt.Errorf("could not sign and build tx bz: %w", err)
	}

	res, err := txs.rpcClient.BroadcastTxSync(ctx, bz)
	if err != nil {
		return fmt.Errorf("error broadcasting sync transaction: %w", err)
	}

	if res.Code == 0 {
		txs.sequence += 1
		return nil
	} else if res.Code == 32 {
		// code 32 is "incorrect account sequence" error
		errInit := txs.Init()
		if errInit != nil {
			return fmt.Errorf("error broadcasting sync transaction: failed to reinit sender: %w", err)
		}
		return fmt.Errorf("error broadcasting sync transaction: tx sender reinitialized, try again, log=%s", res.Log)
	} else {
		return fmt.Errorf("error broadcasting sync transaction with log=%s", res.Log)
	}

}

func (txs *TxSender) SenderAddr() (string, error) {
	info, err := txs.keybase.Key(txs.signKeyName)
	if err != nil {
		return "", fmt.Errorf("could not fetch sender info from keychain with signKeyName=%s: %w", txs.signKeyName, err)
	}

	return info.GetAddress().String(), nil
}

// queryAccount returns BaseAccount for given account address
func (txs *TxSender) queryAccount(ctx context.Context, address string) (*authtypes.BaseAccount, error) {
	request := authtypes.QueryAccountRequest{Address: address}
	req, err := request.Marshal()
	if err != nil {
		return nil, fmt.Errorf("error marshalling query account request for account=%s: %w", address, err)
	}
	simQuery := abci.RequestQuery{
		Path: accountQueryPath,
		Data: req,
	}
	res, err := txs.rpcClient.ABCIQueryWithOptions(ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return nil, fmt.Errorf("error making abci query for account=%s: %w", address, err)
	}

	if res.Response.Code != 0 {
		return nil, fmt.Errorf("error fetching account with address=%s log=%s", address, res.Response.Log)
	}

	var response authtypes.QueryAccountResponse
	if err := response.Unmarshal(res.Response.Value); err != nil {
		return nil, fmt.Errorf("error unmarshalling QueryAccountResponse for account=%s: %w", address, err)
	}

	var account authtypes.BaseAccount
	err = account.Unmarshal(response.Account.Value)

	if err != nil {
		return nil, fmt.Errorf("error unmarshalling BaseAccount for account=%s: %w", address, err)
	}

	return &account, nil
}

func (txs *TxSender) signAndBuildTxBz(txf tx.Factory, msgs []sdk.Msg) ([]byte, error) {
	txBuilder, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction builder: %w", err)
	}

	err = tx.Sign(txf, txs.signKeyName, txBuilder, false)

	if err != nil {
		return nil, fmt.Errorf("error signing transaction: %w", err)
	}

	bz, err := txs.txConfig.TxEncoder()(txBuilder.GetTx())
	return bz, err
}

func (txs *TxSender) calculateGas(ctx context.Context, txf tx.Factory, msgs ...sdk.Msg) (uint64, error) {
	simulation, err := txs.buildSimulationTx(txf, msgs...)
	if err != nil {
		return 0, fmt.Errorf("error building simulation tx: %w", err)
	}
	// We then call the Simulate method on this client.
	simQuery := abci.RequestQuery{
		Path: simulateQueryPath,
		Data: simulation,
	}
	res, err := txs.rpcClient.ABCIQueryWithOptions(ctx, simQuery.Path, simQuery.Data, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return 0, fmt.Errorf("error making abci query for gas calculation: %w", err)
	}

	var simRes txtypes.SimulateResponse

	if err := simRes.Unmarshal(res.Response.Value); err != nil {
		return 0, fmt.Errorf("error unmarshalling simulate response value: %w", err)
	}
	if simRes.GasInfo == nil {
		return 0, fmt.Errorf("no result in simulation response with log=%s code=%d", res.Response.Log, res.Response.Code)
	}

	return uint64(txf.GasAdjustment() * float64(simRes.GasInfo.GasUsed)), nil
}

// buildSimulationTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be built.
func (txs *TxSender) buildSimulationTx(txf tx.Factory, msgs ...sdk.Msg) ([]byte, error) {
	txb, err := txs.baseTxf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, fmt.Errorf("error building unsigned tx for simulation: %w", err)
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: &secp256k1.PubKey{},
		Data: &signing.SingleSignatureData{
			SignMode: txs.baseTxf.SignMode(),
		},
		Sequence: txf.Sequence(),
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, fmt.Errorf("error settings signatures for simulation: %w", err)
	}

	bz, err := txs.txConfig.TxEncoder()(txb.GetTx())
	if err != nil {
		return nil, fmt.Errorf("error encoding transaction: %w", err)
	}
	simReq := txtypes.SimulateRequest{TxBytes: bz}
	return simReq.Marshal()
}
