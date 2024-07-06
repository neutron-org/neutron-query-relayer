package submit

import (
	"context"
	"testing"

	cosmossdk_io_math "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types"
	authtxtypes "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/golang/mock/gomock"
	submit_mock "github.com/neutron-org/neutron-query-relayer/testutil/mocks/submit"
	feemarkettypes "github.com/skip-mev/feemarket/x/feemarket/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTest(t *testing.T, gasMultiplier float64, maxGas float64, gasPrices string, mockClient *submit_mock.MockClient) *TxSender {
	logger, _ := zap.NewDevelopment()
	txConfig := authtxtypes.NewTxConfig(codec.NewProtoCodec(nil), authtxtypes.DefaultSignModes)
	return &TxSender{
		rpcClient:          mockClient,
		txConfig:           txConfig,
		logger:             logger,
		denom:              "testdenom",
		gasPriceMultiplier: gasMultiplier,
		maxGasPrice:        maxGas,
		gasPrices:          gasPrices,
	}
}

func TestQueryDynamicPrice(t *testing.T) {
	tests := []struct {
		name          string
		gasMultiplier float64
		maxGas        float64
		gasPrices     string
		mockResponse  *coretypes.ResultABCIQuery
		expectedPrice string
		expectError   bool
	}{
		{
			name:          "Basic",
			gasMultiplier: 1.0,
			maxGas:        100000000,
			mockResponse: &coretypes.ResultABCIQuery{
				Response: abci.ResponseQuery{
					Code: 0,
					Value: func() []byte {
						gasPrice := feemarkettypes.GasPriceResponse{
							Price: types.DecCoin{
								Denom:  "testdenom",
								Amount: cosmossdk_io_math.LegacyNewDec(123),
							},
						}
						rawGasPrice, _ := gasPrice.Marshal()
						return rawGasPrice
					}(),
				},
			},
			expectedPrice: "123testdenom",
			expectError:   false,
		},
		{
			name:          "Multiply",
			gasMultiplier: 1.1,
			maxGas:        100000000,
			mockResponse: &coretypes.ResultABCIQuery{
				Response: abci.ResponseQuery{
					Code: 0,
					Value: func() []byte {
						gasPrice := feemarkettypes.GasPriceResponse{
							Price: types.DecCoin{
								Denom:  "testdenom",
								Amount: cosmossdk_io_math.LegacyNewDec(123),
							},
						}
						rawGasPrice, _ := gasPrice.Marshal()
						return rawGasPrice
					}(),
				},
			},
			expectedPrice: "135.3testdenom",
			expectError:   false,
		},
		{
			name:          "DefaultGasPrice",
			gasMultiplier: 1.0,
			maxGas:        100.0,
			gasPrices:     "99.0testdenom",
			mockResponse: &coretypes.ResultABCIQuery{
				Response: abci.ResponseQuery{
					Code: 1,
				},
			},
			expectedPrice: "99.0testdenom",
			expectError:   false,
		},
		{
			name:          "MaxGasPrice",
			gasMultiplier: 1.5,
			maxGas:        111.1,
			gasPrices:     "99.0testdenom",
			mockResponse: &coretypes.ResultABCIQuery{
				Response: abci.ResponseQuery{
					Code: 0,
					Value: func() []byte {
						gasPrice := feemarkettypes.GasPriceResponse{
							Price: types.DecCoin{
								Denom:  "testdenom",
								Amount: cosmossdk_io_math.LegacyNewDec(123),
							},
						}
						rawGasPrice, _ := gasPrice.Marshal()
						return rawGasPrice
					}(),
				},
			},
			expectedPrice: "111.1testdenom",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := submit_mock.NewMockClient(ctrl)
			txs := setupTest(t, tt.gasMultiplier, tt.maxGas, tt.gasPrices, mockClient)

			if tt.mockResponse != nil {
				mockClient.EXPECT().
					ABCIQueryWithOptions(gomock.Any(), getPricesQueryPath, gomock.Any(), gomock.Any()).
					Return(tt.mockResponse, nil)
			} else {
				mockClient.EXPECT().
					ABCIQueryWithOptions(gomock.Any(), getPricesQueryPath, gomock.Any(), gomock.Any()).
					Return(nil, nil)
			}

			price, err := txs.getGasPrice(context.Background())
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPrice, price)
			}
		})
	}
}
