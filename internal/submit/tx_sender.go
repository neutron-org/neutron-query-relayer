package submit

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"

	cosmossdk_io_math "cosmossdk.io/math"
	tmtypes "github.com/cometbft/cometbft/types"
	"go.uber.org/zap"

	"cosmossdk.io/api/tendermint/abci"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
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
	feemarkettypes "github.com/skip-mev/feemarket/x/feemarket/types"

	"github.com/neutron-org/neutron-query-relayer/internal/config"
)

const (
	accountQueryPath             = "/cosmos.auth.v1beta1.Query/Account"
	simulateQueryPath            = "/cosmos.tx.v1beta1.Service/Simulate"
	getPricesQueryPath           = "/feemarket.feemarket.v1.Query/GasPrice"
	IncorrectAccountSequenceCode = 32
)

type TxSender struct {
	lock               sync.Mutex
	sequence           uint64
	accountNumber      uint64
	keybase            keyring.Keyring
	baseTxf            tx.Factory
	txConfig           client.TxConfig
	rpcClient          rpcclient.Client
	chainID            string
	signKeyName        string
	gasPrices          string
	gasLimit           uint64
	logger             *zap.Logger
	denom              string
	maxGasPrice        float64
	gasPriceMultiplier float64
}

func TestKeybase(chainID string, keyringRootDir string, cdc codec.Codec) (keyring.Keyring, error) {
	keybase, err := keyring.New(chainID, "test", keyringRootDir, nil, cdc)
	if err != nil {
		return keybase, fmt.Errorf("error creating keybase for chainId=%s and keyringRootDir=%s: %w", chainID, keyringRootDir, err)
	}

	return keybase, nil
}

func NewTxSender(
	ctx context.Context,
	rpcClient rpcclient.Client,
	marshaller codec.ProtoCodecMarshaler,
	keybase keyring.Keyring,
	cfg config.NeutronChainConfig,
	logger *zap.Logger,
	neutronChainID string,
) (*TxSender, error) {
	txConfig := authtxtypes.NewTxConfig(marshaller, authtxtypes.DefaultSignModes)
	baseTxf := tx.Factory{}.
		WithKeybase(keybase).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT).
		WithTxConfig(txConfig).
		WithChainID(neutronChainID).
		WithGasAdjustment(cfg.GasAdjustment).
		WithGasPrices(cfg.GasPrices)

	txs := &TxSender{
		lock:               sync.Mutex{},
		keybase:            keybase,
		txConfig:           txConfig,
		baseTxf:            baseTxf,
		rpcClient:          rpcClient,
		chainID:            neutronChainID,
		signKeyName:        cfg.SignKeyName,
		gasPrices:          cfg.GasPrices,
		gasLimit:           cfg.GasLimit,
		logger:             logger,
		denom:              cfg.Denom,
		maxGasPrice:        cfg.MaxGasPrice,
		gasPriceMultiplier: cfg.GasPriceMultiplier,
	}
	err := txs.refreshAccountInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to init tx sender: %w", err)
	}

	return txs, nil
}

func (txs *TxSender) refreshAccountInfo(ctx context.Context) error {
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
func (txs *TxSender) Send(ctx context.Context, msgs []sdk.Msg) (string, error) {
	txs.lock.Lock()
	defer txs.lock.Unlock()

	gasPrice, err := txs.getGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("error requesting feemarket gas price: %w", err)
	}

	txf := txs.baseTxf.
		WithAccountNumber(txs.accountNumber).
		WithSequence(txs.sequence).
		WithGasPrices(gasPrice)

	gasNeeded, err := txs.calculateGas(ctx, txf, msgs...)
	if err != nil {
		// at this point error code for "incorrect account sequence" is 18 = "invalid request"
		// it's a very common error code to rely on, hence we have to rely on error message
		if strings.Contains(err.Error(), "incorrect account sequence") {
			errInit := txs.refreshAccountInfo(ctx)
			if errInit != nil {
				return "", fmt.Errorf("error calculating gas: failed to reinit sender: %w", errInit)
			}
			txs.logger.Info("sender reinitialized successfully (account sequence reset)")
		}
		return "", fmt.Errorf("error calculating gas: %w", err)
	}

	if txs.gasLimit > 0 && gasNeeded > txs.gasLimit {
		return "", fmt.Errorf("exceeds gas limit: gas needed %d, gas limit %d", gasNeeded, txs.gasLimit)
	}

	txf = txf.
		WithGas(gasNeeded)

	bz, err := txs.signAndBuildTxBz(ctx, txf, msgs)
	if err != nil {
		return "", fmt.Errorf("could not sign and build tx bz: %w", err)
	}

	res, err := txs.rpcClient.BroadcastTxSync(ctx, bz)
	if err != nil {
		return "", fmt.Errorf("error broadcasting sync transaction: %w", err)
	}

	if res.Code == 0 {
		txs.sequence += 1
		return hex.EncodeToString(tmtypes.Tx(bz).Hash()), nil
	}

	if res.Code == IncorrectAccountSequenceCode {
		errInit := txs.refreshAccountInfo(ctx)
		if errInit != nil {
			return "", fmt.Errorf("error broadcasting sync transaction: failed to reinit sender: %w", errInit)
		}
		txs.logger.Info("sender reinitialized successfully (account sequence reset)")
	}
	return "", fmt.Errorf("error broadcasting sync transaction: log=%s", res.Log)
}

func (txs *TxSender) SenderAddr() (string, error) {
	info, err := txs.keybase.Key(txs.signKeyName)
	if err != nil {
		return "", fmt.Errorf("could not fetch sender info from keychain with signKeyName=%s: %w", txs.signKeyName, err)
	}

	addr, err := info.GetAddress()
	if err != nil {
		return "", fmt.Errorf("failed to get addr from pubkey: %w", err)
	}
	return addr.String(), nil
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

func (txs *TxSender) queryDynamicPrice(ctx context.Context) (cosmossdk_io_math.LegacyDec, error) {
	request := feemarkettypes.GasPriceRequest{Denom: txs.denom}
	req, err := request.Marshal()
	if err != nil {
		return cosmossdk_io_math.LegacyZeroDec(), fmt.Errorf("error marshalling query gas prices request for denom=%s: %w", txs.denom, err)
	}
	res, err := txs.rpcClient.ABCIQueryWithOptions(ctx, getPricesQueryPath, req, rpcclient.DefaultABCIQueryOptions)
	if err != nil {
		return cosmossdk_io_math.LegacyZeroDec(), fmt.Errorf("error making abci query: %w", err)
	}

	if res.Response.Code != 0 {
		return cosmossdk_io_math.LegacyZeroDec(), fmt.Errorf("error fetching feemarket gas price for denom=%s", txs.denom)
	}

	var response feemarkettypes.GasPriceResponse
	if err := response.Unmarshal(res.Response.Value); err != nil {
		return cosmossdk_io_math.LegacyZeroDec(), fmt.Errorf("error unmarshalling GasPriceResponse for denom=%s: %w", txs.denom, err)
	}

	gasPrice := response.Price.Amount

	return gasPrice, nil
}

func (txs *TxSender) multiplyGas(gas cosmossdk_io_math.LegacyDec) (string, error) {
	floatGas, err := gas.Float64()
	if err != nil {
		return "", err
	}

	multipliedGas := txs.gasPriceMultiplier * floatGas
	if multipliedGas > txs.maxGasPrice {
		txs.logger.Info("calculating gas price: gas multiplication exceeds max gas price",
			zap.String("gas_before_multiplication", gas.String()),
			zap.Float64("multiplier", txs.gasPriceMultiplier),
			zap.Float64("max_gas_price", txs.maxGasPrice))
		multipliedGas = txs.maxGasPrice
	}

	return strconv.FormatFloat(multipliedGas, 'g', 4, 64), nil
}

// getGasPrice tries to query dynamic price for denom provided in config.
// If successful:
// 1) multiply gas by gasPriceMultiplier (tip validators)
// 2) if result is bigger than maxGasPrice -> return max gas
// if query fails for some reason:
// func returns default gas price[s] from config as well
func (txs *TxSender) getGasPrice(ctx context.Context) (string, error) {
	gasPrice, err := txs.queryDynamicPrice(ctx)
	if err != nil {
		txs.logger.Error("error querying feemarket price", zap.Error(err))
		return txs.gasPrices, nil
	}

	multipliedGas, err := txs.multiplyGas(gasPrice)
	if err != nil {
		return "", err
	}

	return multipliedGas + txs.denom, nil
}

func (txs *TxSender) signAndBuildTxBz(ctx context.Context, txf tx.Factory, msgs []sdk.Msg) ([]byte, error) {
	txBuilder, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to build transaction builder: %w", err)
	}

	err = tx.Sign(ctx, txf, txs.signKeyName, txBuilder, false)

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
