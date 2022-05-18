package chain

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
)

type Codec struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaller        codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func MakeCodecConfig() Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaller := codec.NewProtoCodec(interfaceRegistry)
	return Codec{
		InterfaceRegistry: interfaceRegistry,
		Marshaller:        marshaller,
		TxConfig:          tx.NewTxConfig(marshaller, tx.DefaultSignModes),
		Amino:             codec.NewLegacyAmino(),
	}
}
